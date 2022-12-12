package remote

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/kevmo314/fedtorch/governor/pubsub/local"

	gpupb "github.com/kevmo314/fedtorch/governor/api/go/gpu"
)

type Allocator struct {
	ambient <-chan *gpupb.LeaseResponse
	returns chan *gpupb.LeaseResponse

	l         sync.Mutex
	fulfilled map[string]*gpupb.LeaseResponse

	local *local.Allocator

	wait time.Duration
}

type O struct {
	AmbientTraffic <-chan *gpupb.LeaseResponse

	LocalAllocator *local.Allocator
}

func New(o O, wait time.Duration) *Allocator {
	a := &Allocator{
		ambient: o.AmbientTraffic,

		returns:   make(chan *gpupb.LeaseResponse),
		fulfilled: make(map[string]*gpupb.LeaseResponse),
		local:     o.LocalAllocator,
		wait:      wait,
	}

	go a.listener()
	go a.cleaner()

	return a
}

// Lease attempts to reserve a GPU for the incoming remote lease request.
func (a *Allocator) Lease(req *gpupb.LeaseRequest) (*gpupb.LeaseResponse, error) {
	// Fuzz sleep for a bit in case someone else responds to the same
	// request.
	time.Sleep(time.Duration((1 + rand.Float64()) * float64(a.wait)))

	a.l.Lock()
	defer a.l.Unlock()

	// Check local cache to see if this request has already been fulfilled.
	if _, ok := a.fulfilled[req.GetToken()]; ok {
		return nil, fmt.Errorf("request already filled")
	}

	// Attempt to reserve local GPU.
	resp, err := a.local.Lease(req)
	if err != nil {
		return nil, err
	}

	go func(resp *gpupb.LeaseResponse) {
		time.Sleep(time.Until(resp.GetLease().GetExpiration().AsTime()))
		a.returns <- resp
	}(resp)

	return resp, nil
}

func (a *Allocator) listener() {
	for resp := range a.ambient {
		a.l.Lock()

		a.fulfilled[resp.GetLease().GetToken()] = resp

		a.l.Unlock()

		go func(resp *gpupb.LeaseResponse) {
			time.Sleep(time.Until(resp.GetLease().GetExpiration().AsTime()))
			a.returns <- resp
		}(resp)
	}
}

func (a *Allocator) cleaner() {
	for resp := range a.returns {
		func() {
			a.l.Lock()
			defer a.l.Unlock()

			m, ok := a.fulfilled[resp.GetLease().GetToken()]
			if !ok {
				return
			}

			if resp.GetRequestor() != m.GetRequestor() {
				return
			}

			// Do not check expiration time.
			delete(a.fulfilled, resp.GetLease().GetToken())
		}()
	}
}
