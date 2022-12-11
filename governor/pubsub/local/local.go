package local

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	gpupb "github.com/kevmo314/fedtorch/governor/api/go/gpu"
	tpb "google.golang.org/protobuf/types/known/timestamppb"
)

const (
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

type LocalAllocator struct {
	// gpus is immutable after construction
	gpus []*gpupb.GPU

	l      sync.Mutex
	leases map[int32]*gpupb.Lease

	returnGPU chan *gpupb.Lease
	grace     time.Duration
}

func New(gpus []*gpupb.GPU, grace time.Duration) *LocalAllocator {
	a := &LocalAllocator{
		gpus:      gpus,
		leases:    make(map[int32]*gpupb.Lease),
		returnGPU: make(chan *gpupb.Lease),
		grace:     grace,
	}
	go a.daemon()
	return a
}

func (a *LocalAllocator) Get(x int32) *gpupb.GPU { return a.gpus[x] }

func (a *LocalAllocator) daemon() {
	for l := range a.returnGPU {
		func() {
			a.l.Lock()
			defer a.l.Unlock()

			m, ok := a.leases[l.GetId()]
			if !ok {
				return
			}

			if l.GetToken() != m.GetToken() {
				return
			}

			// Do not check expiration time -- it is possible for us
			// to have an early return event.
			delete(a.leases, l.GetId())
		}()
	}
}

func (a *LocalAllocator) Reserve(lease time.Duration) (*gpupb.Lease, error) {
	t := generateToken()
	expiration := time.Now().Add(lease).Add(a.grace)
	g, err := func() (*gpupb.Lease, error) {
		a.l.Lock()
		defer a.l.Unlock()

		for _, g := range a.gpus {
			l, ok := a.leases[g.GetId()]
			if !ok || time.Now().After(l.GetExpiration().AsTime()) {
				m := &gpupb.Lease{
					Token:      t,
					Id:         g.GetId(),
					Expiration: tpb.New(expiration),
				}
				a.leases[g.GetId()] = m

				return m, nil
			}
		}

		return nil, fmt.Errorf("no local GPU available")
	}()

	if err != nil {
		go func() {
			time.After(time.Until(expiration))
			a.returnGPU <- g
		}()
	}
	return g, err
}
