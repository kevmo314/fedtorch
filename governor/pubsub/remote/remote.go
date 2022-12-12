package remote

import (
	"math/rand"
	"sync"
	"time"

	"github.com/kevmo314/fedtorch/governor/pubsub/local"

	gpupb "github.com/kevmo314/fedtorch/governor/api/go/gpu"
)

type Allocator struct {
	ambient   <-chan *gpupb.Fulfillment
	responses chan<- *gpupb.Fulfillment
	returns   chan string

	l         sync.Mutex
	fulfilled map[string]time.Time

	local *local.Allocator

	wait time.Duration
}

type O struct {
	AmbientTraffic <-chan *gpupb.Fulfillment
	Responses      chan<- *gpupb.Fulfillment

	LocalAllocator *local.Allocator
	WaitTime       time.Duration
}

func New(o O) *Allocator {
	a := &Allocator{
		ambient:   o.AmbientTraffic,
		responses: o.Responses,

		returns:   make(chan string),
		fulfilled: make(map[string]time.Time),
		local:     o.LocalAllocator,
		wait:      o.WaitTime,
	}

	go a.listener()
	go a.cleaner()

	return a
}

func (a *Allocator) Lease(req *gpupb.LeaseRequest) {
	// Fuzz sleep for a bit in case someone else responds to the same
	// request.
	time.Sleep(time.Duration((1 + rand.Float64()) * float64(a.wait)))

	a.l.Lock()
	defer a.l.Unlock()

	// Check local cache to see if this request has already been fulfilled.
	if _, ok := a.fulfilled[req.GetToken()]; ok {
		return nil
	}

	// Attempt to reserve local GPU.
	

	if l := func() *gpupb.Lease {
		a.l.Lock()
		defer a.l.Unlock()

		if _, ok := a.fulfilled[r.GetToken()]; ok {
			return nil
		}

		l, err := a.local.Reserve(r.GetLease().AsDuration())
		if err != nil {
			return nil
		}

		return l
	}(); l != nil {
		a.responses <- &gpupb.Fulfillment{
			Requestor: r.GetRequestor(),

			Gpu:   a.local.Get(l.GetId()),
			Lease: l,
		}

		go func(l *gpupb.Lease) {
			time.Sleep(time.Until(l.GetExpiration().AsTime()))
			a.returns <- l.GetToken()
		}(l)
	}
}

func (a *Allocator) listener() {
	for r := range a.ambient {
		a.l.Lock()

		a.fulfilled[r.GetToken()] = r.GetLease().GetExpiration().AsTime()

		a.l.Unlock()

		go func(r *gpupb.Fulfillment) {
			time.Sleep(time.Until(r.GetLease().GetExpiration().AsTime()))
			a.returns <- r.GetToken()
		}(r)
	}
}

func (a *Allocator) cleaner() {
	for x := range a.returns {
		a.l.Lock()

		// Do not check expiration time.
		delete(a.fulfilled, x)

		a.l.Unlock()
	}
}
