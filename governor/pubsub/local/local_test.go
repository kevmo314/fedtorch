package local

import (
	"testing"
	"time"

	gpupb "github.com/kevmo314/fedtorch/governor/api/go/gpu"
	tpb "google.golang.org/protobuf/types/known/timestamppb"
)

func TestReserve(t *testing.T) {
	configs := []struct {
		name string
		a    *Allocator
		succ bool
	}{
		{
			name: "Empty",
			a:    New(nil, 0),
			succ: false,
		},
		{
			name: "Single",
			a: New([]*gpupb.GPU{
				&gpupb.GPU{
					Id: 100,
				},
			}, 0),
			succ: true,
		},
		{
			name: "Full",
			a: &Allocator{
				gpus: []*gpupb.GPU{
					&gpupb.GPU{
						Id: 100,
					},
				},
				leases: map[int32]*gpupb.Lease{
					100: &gpupb.Lease{
						Id:         100,
						Expiration: tpb.New(time.Now().Add(time.Hour)),
					},
				},
			},
			succ: false,
		},
	}

	for _, c := range configs {
		t.Run(c.name, func(t *testing.T) {
			l, err := c.a.Reserve(time.Minute)
			if !c.succ && err == nil {
				t.Errorf("Reserve() unexpectedly succeeded: %v", l)
			} else if c.succ && err != nil {
				t.Errorf("Reserve() unexpectedly failed: %v", err)
			}
		})
	}
}

func TestReturn(t *testing.T) {
	a := New([]*gpupb.GPU{
		&gpupb.GPU{
			Id: 100,
		},
	}, 0)

	l, err := a.Reserve(time.Second)
	if err != nil {
		t.Fatalf("Reserve unexpectedly failed: %v", err)
	}

	time.Sleep(time.Until(l.GetExpiration().AsTime()))

	if _, err := a.Reserve(time.Second); err != nil {
		t.Errorf("Reserve unexpectedly failed: %v", err)
	}
}
