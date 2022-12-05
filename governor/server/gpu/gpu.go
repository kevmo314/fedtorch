package gpu

import (
	"sync"
	"time"

	"github.com/kevmo314/fedtorch/governor/p2p"
	"gorgonia.org/cu"

	gpupb "github.com/kevmo314/fedtorch/governor/api/go/gpu"
	tpb "google.golang.org/protobuf/types/known/timestamppb"
)

type L struct {
	gpusL sync.Mutex
	gpus  []*gpupb.GPU

	returned chan *gpupb.GPU
	dht      *p2p.Store
}

func New(dht *p2p.Store) *L {
	l := &L{
		gpus: get(dht.Host()),
		dht:  dht,
	}
	if len(l.gpus) > 0 {
		l.dht.Announce(p2p.TopicHasGPUCapacity)
	}
	return l
}

func (l *L) AllocateGPU(n int) []*gpupb.GPU {
	tok := "random-string"

	l.gpusL.Lock()
	defer l.gpusL.Unlock()

	free := getFree(l.gpus)
	reserved := make([]*gpupb.GPU, 0, len(free))

	for _, g := range free {
		g.AllocationToken = tok

		// LeaseExpiration is currently not checked. Once we check this, we will
		// need another pipeline to asynchronously update the DHT.
		//
		// TODO(minkezhang): Unallocate after length of time if not
		// actually claimed.
		g.LeaseExpiration = tpb.New(time.Now().Add(time.Minute))

		reserved = append(reserved, g)
		if len(reserved) >= n {
			break
		}
	}

	if len(reserved) == len(free) {
		l.dht.Revoke(p2p.TopicHasGPUCapacity)
	}

	return reserved
}

func (l *L) ReserveGPU(gpu int, token string) bool {
	l.gpusL.Lock()
	defer l.gpusL.Unlock()

	if gpu > len(l.gpus) {
		return false
	}

	// TODO(minkezhang): Also check for lease expiration time.
	if l.gpus[gpu].GetAllocationToken() != token {
		return false
	}

	return true
}

func (l *L) ReleaseGPU(gpu int, token string) bool {
	l.gpusL.Lock()
	defer l.gpusL.Unlock()

	closed := len(getFree(l.gpus))

	if gpu > len(l.gpus) {
		return false
	}

	g := l.gpus[gpu]
	if validate(g, token) {
		g.AllocationToken = ""
		return true
	}

	// At least one GPU is free when it was not previously -- let others now
	// we are available for more work.
	if closed == 0 {
		l.dht.Announce(p2p.TopicHasGPUCapacity)
	}

	return false
}

func get(host string) []*gpupb.GPU {
	n, err := cu.NumDevices()
	if err != nil {
		return nil
	}

	devices := make([]*gpupb.GPU, 0, n)
	for d := 0; d < n; d++ {
		dev := cu.Device(d)
		name, _ := dev.Name()
		cr, _ := dev.Attribute(cu.ClockRate)
		mem, _ := dev.TotalMem()
		g := &gpupb.GPU{
			Host:      host,
			DeviceId:  int32(d),
			Name:      name,
			ClockRate: int32(cr),
			Memory:    mem,
		}
		devices = append(devices, g)
	}

	return devices
}

func getFree(gpus []*gpupb.GPU) []*gpupb.GPU {
	free := make([]*gpupb.GPU, 0, len(gpus))
	for _, g := range gpus {
		if !allocated(g) {
			free = append(free, g)
		}
	}
	return free
}

func validate(g *gpupb.GPU, token string) bool {
	return allocated(g) && g.GetAllocationToken() == token
}

func allocated(g *gpupb.GPU) bool {
	return g.GetAllocationToken() != "" // && time.Now().Before(g.LeaseExpiration().AsTime())
}
