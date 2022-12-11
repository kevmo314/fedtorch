package local

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	gpupb "github.com/kevmo314/fedtorch/governor/api/go/gpu"
	tpb "google.golang.org/protobuf/types/known/timestamppb"
)

var (
	grace = time.Minute
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
	l      sync.Mutex
	gpus   []*gpupb.GPU
	leases map[int32]*gpupb.Lease

	returnGPU chan *gpupb.Lease
}

func New(gpus []*gpupb.GPU) *LocalAllocator {
	return &LocalAllocator{
		gpus:      gpus,
		leases:    make(map[int32]*gpupb.Lease),
		returnGPU: make(chan *gpupb.Lease),
	}
}

func (a *LocalAllocator) Reserve(lease time.Duration) (*gpupb.Lease, error) {
	t := generateToken()
	expiration := time.Now().Add(lease)
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
			time.After(time.Until(expiration) + grace)
			a.returnGPU <- g
		}()
	}
	return g, err
}
