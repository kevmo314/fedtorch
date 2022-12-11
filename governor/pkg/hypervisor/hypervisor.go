package hypervisor

import (
	"fmt"
	"net"
	"os"
	"os/exec"
)

type Hypervisor exec.Cmd

func NewHypervisor(master net.Addr, id string, total int, script string) (*Hypervisor, error) {
	// write script to temp file
	f, err := os.CreateTemp("", "hypervisor")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString(script); err != nil {
		return nil, fmt.Errorf("failed to write script to temp file: %w", err)
	}
	if err := f.Close(); err != nil {
		return nil, fmt.Errorf("failed to close temp file: %w", err)
	}
	cmd := exec.Command(
		"docker", "run", "-it", "--rm", "--gpus", "all", "--network", "host", "nvcr.io/nvidia/pytorch:22.01-py3", "torchrun",
		fmt.Sprintf("--nnodes=1:%d", total),
		"--nproc_per_node=1",
		fmt.Sprintf("--rdzv_id=%s", id),
		"--rdzv_backend=c10d",
		fmt.Sprintf("--rdzv_endpoint=%s", master.String()),
		f.Name(),
	)
	return (*Hypervisor)(cmd), nil
}
