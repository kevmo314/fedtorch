package pubsub

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/kevmo314/fedtorch/governor/pubsub/local"
	"github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
	"google.golang.org/protobuf/proto"

	gpupb "github.com/kevmo314/fedtorch/governor/api/go/gpu"
	dpb "google.golang.org/protobuf/types/known/durationpb"
)

const (
	GPURequestTopic     = "GPU_REQUEST"
	GPUFulfillmentTopic = "GPU_FULFILLMENT"

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

func pub[T proto.Message](ctx context.Context, pubsub *pubsub.PubSub, topic string) chan<- T {
	t, err := pubsub.Join(topic)
	if err != nil {
		panic(fmt.Sprintf("cannot join topic %v: %v", topic, err))
	}

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

			if !f(pb) {
				ch <- pb
			}
		}
	}()
	return ch
}

type GPUAllocator struct {
	// id is the host ID as provided by the p2plib host.Host.ID(). The
	// allocator does not track the host, and therefore this data needs to
	// be passed in explicitly.
	id peer.ID

	pubsub      *pubsub.PubSub
	request     *pubsub.Topic
	fulfillment *pubsub.Topic

	requestSub     *pubsub.Subscription
	fulfillmentSub *pubsub.Subscription

	local *local.Allocator
}

type O struct {
	GovernorAddress string
	PubSub          *pubsub.PubSub
	P2PID           peer.ID
	GPUs            []*gpupb.GPU
}

// New constructs a GPUAllocator daemon.
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
// Motivated by https://medium.com/rahasak/libp2p-pubsub-with-golang-495539e6aae1.
func New(ctx context.Context, o O) *GPUAllocator {
	request, err := o.PubSub.Join(GPURequestTopic)
	if err != nil {
		panic(fmt.Sprintf("could not join GPU request topic %v: %v", GPURequestTopic, err))
	}
	fulfillment, err := o.PubSub.Join(GPUFulfillmentTopic)
	if err != nil {
		panic(fmt.Sprintf("could not join GPU fulfillment topic %v: %v", GPUFulfillmentTopic, err))
	}

	requestSub, err := request.Subscribe(pubsub.WithBufferSize(0))
	if err != nil {
		panic(fmt.Sprintf("could not subscribe to the GPU request topic %v: %v", GPURequestTopic, err))
	}

	fulfillmentSub, err := fulfillment.Subscribe(pubsub.WithBufferSize(0))
	if err != nil {
		panic(fmt.Sprintf("could not subscribe to the GPU fulfillment response topic %v: %v", GPUFulfillmentTopic, err))
	}

	a := &GPUAllocator{
		id:     o.P2PID,
		pubsub: o.PubSub,

		request:     request,
		fulfillment: fulfillment,

		requestSub:     requestSub,
		fulfillmentSub: fulfillmentSub,

		local: local.New(o.GPUs, time.Minute),
	}

	fulfillmentMonitorSub, err := fulfillment.Subscribe(pubsub.WithBufferSize(0))
	if err != nil {
		panic(fmt.Sprintf("could not subscribe to the GPU fulfillment monitoring topic %v: %v", GPUFulfillmentTopic, err))
	}

	go a.fulfillmentDaemon(ctx, fulfillmentMonitorSub)

	return a
}

// fulfillmentDaemon listens on the network for remote nodes that need GPUs.
// Because this is a distributed system, it is possible for multiple nodes to
// receive the same remote node request. In order to limit the amount of
// duplicated reservations, we will do a random backoff.
//
// TODO(minkezhang): Use a Allocate / Reserve / Free model instead in v2 in
// which the gRPC server explicitly checks out a GPU for some period of time;
// in the meantime, temporarily lease out the GPU for ~5min.
func (a *GPUAllocator) fulfillmentDaemon(ctx context.Context, monitor *pubsub.Subscription) {
	for {
		msg, err := a.requestSub.Next(ctx)
		if err != nil {
			continue
		}

		req := &gpupb.Request{}
		if err := proto.Unmarshal(msg.Data, req); err != nil {
			continue
		}

		// Wait some time and see if another fulfillment has already
		// happened.
		n := int(rand.Int31n(10) + 10)
		waitCtx, cancel := context.WithDeadline(ctx, time.Now().Add(time.Duration(n*int(time.Second))))
		if !func(ctx context.Context) bool {
			defer cancel()

			for {
				msg, err := monitor.Next(ctx)
				if err != nil {
					return true
				}

				f := &gpupb.Fulfillment{}
				if err := proto.Unmarshal(msg.Data, f); err != nil {
					continue
				}

				if peer.ID(f.GetRequestor()) == peer.ID(req.GetRequestor()) {
					if f.GetToken() == req.GetToken() {
						return false
					}
				}
			}

			// No fulfillment messages have occurred yet.
			return true
		}(waitCtx) {
			continue
		}

		// Local reservation requests are not published.
		if peer.ID(req.GetRequestor()) != a.id {
			l, err := a.local.Reserve(req.GetLease().AsDuration())
			if err == nil {
				f, err := proto.Marshal(&gpupb.Fulfillment{
					Requestor: req.GetRequestor(),
					Token:     req.GetToken(),
					Gpu:       a.local.Get(l.GetId()),
				})
				if err != nil {
					continue
				}
				a.fulfillment.Publish(ctx, f)
			}
		}
	}
}

func (a *GPUAllocator) reserveRemote(ctx context.Context, lease time.Duration) (*gpupb.GPU, error) {
	token := generateToken()

	req, err := proto.Marshal(&gpupb.Request{
		Requestor: string(a.id),
		Lease:     dpb.New(lease),
		Token:     token,
	})
	if err != nil {
		return nil, err
	}
	err = nil

	ctx, cancel := context.WithDeadline(ctx, time.Now().Add(time.Minute))

	var g *gpupb.GPU
	go func(ctx context.Context) {
		// cancelling here means the next request.Publish call may be
		// cut off early. This is probably okay, but will incur some
		// non-zero reservation penalty (i.e. a useless reservation on
		// the network).
		//
		// TODO(minkezhang): Rewrite the network code to be more
		// stateful.
		defer cancel()
		for {
			msg, terr := a.fulfillmentSub.Next(ctx)
			if terr != nil {
				err = terr
				return
			}

			if msg.ReceivedFrom == a.id {
				continue
			}

			f := &gpupb.Fulfillment{}
			if terr := proto.Unmarshal(msg.Data, f); terr != nil {
				continue
			}

			// A fulfillment request has been made on behalf of the
			// local node.
			if peer.ID(f.GetRequestor()) == a.id && f.GetToken() == token {
				g = f.GetGpu()
				return
			}
		}
	}(ctx)

	a.request.Publish(ctx, req)

	<-ctx.Done()

	return g, err
}

// Reserve allocates some GPU on a local or remote host for the specified amount
// of time. This is called by the governor when planning for a workload.
func (a *GPUAllocator) Reserve(ctx context.Context, lease time.Duration) (*gpupb.GPU, error) {
	if l, err := a.local.Reserve(lease); err == nil {
		return a.local.Get(l.GetId()), nil
	}
	return a.reserveRemote(ctx, lease)
}
