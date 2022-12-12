package local

import (
	"fmt"
	"sync"
	"time"

	gpupb "github.com/kevmo314/fedtorch/governor/api/go/gpu"
	tpb "google.golang.org/protobuf/types/known/timestamppb"
)

type Allocator struct {
	// gpus is immutable after construction
	gpus []*gpupb.GPU

	l      sync.Mutex
	leases map[int32]*gpupb.Lease

	returnGPU chan *gpupb.Lease
	grace     time.Duration
}

func New(gpus []*gpupb.GPU, grace time.Duration) *Allocator {
	a := &Allocator{
		gpus:      gpus,
		leases:    make(map[int32]*gpupb.Lease),
		returnGPU: make(chan *gpupb.Lease),
		grace:     grace,
	}
	go a.daemon()
	return a
}

func (a *Allocator) Get(x int32) *gpupb.GPU { return a.gpus[x] }

func (a *Allocator) daemon() {
	for l := range a.returnGPU {
		func() {
			a.l.Lock()
			defer a.l.Unlock()

			m, ok := a.leases[l.GetGpu().GetId()]
			if !ok {
				return
			}

			if l.GetToken() != m.GetToken() {
				return
			}

			// Do not check expiration time -- it is possible for us
			// to have an early return event.
			delete(a.leases, l.GetGpu().GetId())
		}()
	}
}

func (a *Allocator) Lease(req *gpupb.LeaseRequest) (*gpupb.LeaseResponse, error) {
	expiration := time.Now().Add(req.GetDuration().AsDuration()).Add(a.grace)
	l, err := func() (*gpupb.Lease, error) {
		a.l.Lock()
		defer a.l.Unlock()

		for _, g := range a.gpus {
			l, ok := a.leases[g.GetId()]
			if !ok || time.Now().After(l.GetExpiration().AsTime()) {
				m := &gpupb.Lease{
					Token:      req.GetToken(),
					Gpu:        g,
					Expiration: tpb.New(expiration),
				}
				a.leases[g.GetId()] = m

				return m, nil
			}
		}

		return nil, fmt.Errorf("no local GPU available")
	}()

	if err != nil {
		go func(l *gpupb.Lease) {
			time.Sleep(time.Until(expiration))
			a.returnGPU <- l
		}(l)
	}
	return &gpupb.LeaseResponse{
		Requestor: req.GetRequestor(),
		Lease: l,
	}, err
}
