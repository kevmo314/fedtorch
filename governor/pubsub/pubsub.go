package pubsub

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/kevmo314/fedtorch/governor/pubsub/local"
	"github.com/kevmo314/fedtorch/governor/pubsub/remote"
	"github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
	"google.golang.org/protobuf/proto"

	gpupb "github.com/kevmo314/fedtorch/governor/api/go/gpu"
)

const (
	LeaseRequestTopic  = "GPU_REQUEST"
	LeaseResponseTopic = "GPU_FULFILLMENT"

	tokenLength = 64
	tokenSet    = "abcdefghijklmnopqrstuvwxyz" + "ABCDEFGHIJKLMNOPQRSTUVWXYZ" + "0123456789"
)

func generateToken() string {
	b := make([]byte, tokenLength)
	for i := range b {
		b[i] = tokenSet[rand.Intn(len(tokenSet))]
	}
	return string(b)
}

func pub[T proto.Message](ctx context.Context, t *pubsub.Topic) chan<- T {
	ch := make(chan T)
	go func() {
		for msg := range ch {
			data, err := proto.Marshal(msg)
			if err != nil {
				continue
			}
			t.Publish(ctx, data)
		}
	}()
	return ch
}

func sub[T proto.Message](ctx context.Context, t *pubsub.Topic, f func(pb T) bool) <-chan T {
	s, err := t.Subscribe(pubsub.WithBufferSize(0))
	if err != nil {
		panic(fmt.Sprintf("cannot subscribe to topic %v: %v", t.String(), err))
	}

	ch := make(chan T)
	go func() {
		for {
			msg, err := s.Next(ctx)
			if err != nil {
				return
			}

			var pb T
			if err := proto.Unmarshal(msg.Data, pb); err != nil {
				continue
			}

			if f(pb) {
				ch <- pb
			}
		}
	}()
	return ch
}

type O struct {
	GovernorAddress string
	PubSub          *pubsub.PubSub
	PeerID          peer.ID
	GPUs            []*gpupb.GPU
}

type Allocator struct {
	reqPub  chan<- *gpupb.LeaseRequest
	reqSub  <-chan *gpupb.LeaseRequest
	respPub chan<- *gpupb.LeaseResponse
	respSub <-chan *gpupb.LeaseResponse

	// responses keeps track of requests issued locally which have returned
	// from a remote fulfillment. This struct listens to the respSub
	// channel.
	responses map[string]*gpupb.LeaseResponse
	l         sync.Mutex
	clean     chan string

	remote *remote.Allocator
	local  *local.Allocator

	timeout time.Duration
}

// New constructs a Allocator daemon.
//
// The input pubsub instance is assumed to have already been started, as is the
// governor.
//
// The local governer will track this GPUAllocator instance, and estimate how
// much resources will need to be reserved given an incoming workload. For each
// GPU which needs to be called, the governor will call
//
//	gpu, err := a.Reserve(ctx)
//
// A non-nil error will be returned if there was a problem reserving a GPU. The
// governor may call Reserver() multiple times concurrently. The returned GPU
// may be remote. The governor should negotiate with the remote governor on how
// exactly to use the reserved GPU.
//
// Motivated by
// https://medium.com/rahasak/libp2p-pubsub-with-golang-495539e6aae1.
func New(ctx context.Context, o O, timeout time.Duration) *Allocator {
	const fuzz = 15 * time.Second
	if timeout < 2*fuzz {
		panic(fmt.Sprintf("remote fulfillment request timeout %v does not account for backoff fuzzing time %v", timeout, 2*fuzz))
	}

	requestT, err := o.PubSub.Join(LeaseRequestTopic)
	if err != nil {
		panic(fmt.Sprintf("cannot join request topic %v: %v", LeaseRequestTopic, err))
	}

	responseT, err := o.PubSub.Join(LeaseResponseTopic)
	if err != nil {
		panic(fmt.Sprintf("cannot join response topic %v: %v", LeaseResponseTopic, err))
	}

	localAllocator := local.New(o.GPUs, time.Minute)

	a := &Allocator{
		reqPub: pub[*gpupb.LeaseRequest](ctx, requestT),
		reqSub: sub[*gpupb.LeaseRequest](ctx, requestT, func(pb *gpupb.LeaseRequest) bool {
			return peer.ID(pb.GetRequestor()) != o.PeerID
		}),

		respPub: pub[*gpupb.LeaseResponse](ctx, responseT),
		respSub: sub[*gpupb.LeaseResponse](ctx, responseT, func(pb *gpupb.LeaseResponse) bool {
			return peer.ID(pb.GetRequestor()) == o.PeerID
		}),

		remote: remote.New(remote.O{
			AmbientTraffic: sub[*gpupb.LeaseResponse](ctx, responseT, func(pb *gpupb.LeaseResponse) bool {
				return peer.ID(pb.GetRequestor()) != o.PeerID
			}),
		}, fuzz),
		local:   localAllocator,
		timeout: timeout,
	}

	go a.daemon()
	go a.cleaner()
	go a.listener()

	return a
}

func (a *Allocator) daemon() {
	for req := range a.reqSub {
		resp, err := a.remote.Lease(req)
		if err != nil {
			continue
		}

		a.respPub <- resp
	}
}

func (a *Allocator) listener() {
	for resp := range a.respSub {
		a.l.Lock()

		a.responses[resp.GetLease().GetToken()] = resp

		a.l.Unlock()

		go func(resp *gpupb.LeaseResponse) {
			time.Sleep(time.Until(resp.GetLease().GetExpiration().AsTime()))
			a.clean <- resp.GetLease().GetToken()
		}(resp)
	}
}

func (a *Allocator) cleaner() {
	for x := range a.clean {
		a.l.Lock()

		delete(a.responses, x)

		a.l.Unlock()
	}
}

// Lease fulfills a local host's allocation request.
func (a *Allocator) Lease(req *gpupb.LeaseRequest) (*gpupb.LeaseResponse, error) {
	resp, err := a.local.Lease(req)
	if err == nil {
		return resp, nil
	}

	// Try a remote lease request.
	if err != nil {
		select {
		case <-time.After(a.timeout):
			return nil, fmt.Errorf("could not write GPU lease request to the network")
		case a.reqPub <- req:
		}

		// Wait for a remote fulfillment.
		timeout := time.Now().Add(a.timeout)
		for time.Now().Before(timeout) {
			if resp, ok := func() (*gpupb.LeaseResponse, bool) {
				a.l.Lock()
				defer a.l.Unlock()
				resp, ok := a.responses[req.GetToken()]
				return resp, ok
			}(); ok {
				return resp, nil
			}
			time.Sleep(time.Second)
		}
	}
	return nil, fmt.Errorf("could not find a free GPU on the network")
}
