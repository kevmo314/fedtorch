package server

import (
	"context"

	"github.com/kevmo314/fedtorch/governor/p2p"
	"github.com/kevmo314/fedtorch/governor/server/gpu"

	gpb "github.com/kevmo314/fedtorch/governor/api/go/api"
)

type S struct {
	p2p *p2p.Store

	gpus *gpu.L
}

type O struct {
	Address string
	Port    int
}

func New(o O) *S {
	// N.B.: We will need two ports, one for DHT and one for gRPC. Need to
	// think of a way to map these.
	dht := p2p.New(p2p.O{
		Address: o.Address,
		Port:    o.Port,
	})
	return &S{
		p2p: dht,
		// N.B.: This will break, as gpu.New will attempt to call
		// p2p.Announce. Need to be clearer on when p2p.Store starts.
		gpus: gpu.New(dht),
	}
}

// TODO(minkezhang): Use a reservation pipeline architecture instead.
//
//  r := <-chan struct{
//    Host     string
//    DeviceID int
//  }

func (s *S) InternalAllocateGPU(ctx context.Context, req *gpb.InternalAllocateGPURequest) *gpb.InternalAllocateGPUResponse {
	resp := &gpb.InternalAllocateGPUResponse{}
	resp.Gpus = append(resp.Gpus, s.gpus.AllocateGPU(int(req.GetCapacity()))...)

	// TODO(minkezhang): Call DHT to fulfil gap.
	return resp
}

func (s *S) Start() error {
	if err := s.p2p.Start(); err != nil {
		return err
	}
	return nil
}

func (s *S) Stop() { s.p2p.Stop() }
