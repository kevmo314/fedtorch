package hypervisor

import (
	"context"
)

type H interface {
	// RequestAllocateGPU returns a list of currently free GPU IDs. These
	// GPUs may be allocated between this call and ReserveGPU.
	RequestAllocateGPU(n int) (string, []int)

	// ReserveGPU will lock a GPU as used. FreeGPU must be called upon a
	// context cancel. The method returns false if the GPU is already in
	// use.
	ReserveGPU(ctx context.Context, tok string, n int) bool

	FreeGPU(n int) bool
}
