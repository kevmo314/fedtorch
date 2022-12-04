package gpu

import (
	"gorgonia.org/cu"
)

type GPU interface {
	DeviceNumber() int
	Name() string
	Memory() int64
	ClockRate() int
}

type gpu struct {
	deviceNumber int
	name         string
	memoryBytes  int64
	clockRateHz  int
}

func (g gpu) DeviceNumber() int { return g.deviceNumber }
func (g gpu) Name() string      { return g.name }
func (g gpu) Memory() int64     { return g.memoryBytes }
func (g gpu) ClockRate() int    { return g.clockRateHz }

func Get() []GPU {
	n, err := cu.NumDevices()
	if err != nil {
		return nil
	}

	devices := make([]GPU, 0, n)
	for d := 0; d < n; d++ {
		dev := cu.Device(d)
		name, _ := dev.Name()
		cr, _ := dev.Attribute(cu.ClockRate)
		mem, _ := dev.TotalMem()
		devices = append(devices, gpu{
			deviceNumber: d,
			name:         name,
			clockRateHz:  cr,
			memoryBytes:  mem,
		})
	}

	return devices
}
