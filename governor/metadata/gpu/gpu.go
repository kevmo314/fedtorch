package gpu

import (
	"gorgonia.org/cu"

	gpupb "github.com/kevmo314/fedtorch/governor/api/go/gpu"
)

func Generate(host string) []*gpupb.GPU {
	n, err := cu.NumDevices()
	if err != nil {
		return nil
	}

	var devices []*gpupb.GPU
	for d := 0; d < n; d++ {
		dev := cu.Device(d)
		name, _ := dev.Name()
		cr, _ := dev.Attribute(cu.ClockRate)
		mem, _ := dev.TotalMem()
		g := &gpupb.GPU{
			Addr:      host,
			Id:        int32(d),
			Name:      name,
			ClockRate: int32(cr),
			Memory:    mem,
		}
		devices = append(devices, g)
	}

	return devices
}
