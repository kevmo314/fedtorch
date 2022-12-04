package server

import (
	"context"

	"github.com/kevmo314/fedtorch/governor/p2p"

	gpb "github.com/kevmo314/fedtorch/governor/api/go/api"
)

type S struct {
	p2p *p2p.Store
}

type O struct {
	Address string
	Port    int
}

func New(o O) *S {
	return &S{
		p2p: p2p.New(p2p.O{
			Address: o.Address,
			Port:    o.Port,
		}),
	}
}

func (s *S) InternalAllocate(ctx context.Context, req *gpb.InternalAllocateRequest) *gpb.InternalAllocateResponse {
	return nil
}

func (s *S) Run() error { return s.p2p.Run() }
func (s *S) Stop()      { s.p2p.Stop() }
