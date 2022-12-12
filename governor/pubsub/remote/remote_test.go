package remote

import (
	"testing"
	"time"

	"github.com/kevmo314/fedtorch/governor/pubsub/local"

	gpupb "github.com/kevmo314/fedtorch/governor/api/go/gpu"
	dpb "google.golang.org/protobuf/types/known/durationpb"
	tpb "google.golang.org/protobuf/types/known/timestamppb"
)

func TestLease(t *testing.T) {
	configs := []struct {
		name string
		a    *Allocator
		req  *gpupb.LeaseRequest
		want bool
	}{
		{
			name: "Full",
			a: New(O{
				AmbientTraffic: make(chan *gpupb.LeaseResponse),
				LocalAllocator: local.New(nil, 0),
			}, 0),
			req: &gpupb.LeaseRequest{
				Requestor: "some-request-host",
				Token:     "some-token",
				Duration:  dpb.New(time.Second),
			},
			want: false,
		},
		{
			name: "Allocate",
			a: New(O{
				AmbientTraffic: make(chan *gpupb.LeaseResponse),
				LocalAllocator: local.New([]*gpupb.GPU{
					&gpupb.GPU{
						Id: 100,
					},
				}, 0),
			}, 0),
			req: &gpupb.LeaseRequest{
				Requestor: "some-request-host",
				Token:     "some-token",
				Duration:  dpb.New(time.Second),
			},
			want: true,
		},
		{
			name: "AlreadyLeased",
			a: &Allocator{
				fulfilled: map[string]*gpupb.LeaseResponse{
					"some-token": &gpupb.LeaseResponse{
						Requestor: "some-request-host",
					},
				},
				local: local.New([]*gpupb.GPU{
					&gpupb.GPU{
						Id: 100,
					},
				}, 0),
			},
			req: &gpupb.LeaseRequest{
				Requestor: "some-request-host",
				Token:     "some-token",
				Duration:  dpb.New(time.Second),
			},
			want: false,
		},
	}

	for _, c := range configs {
		t.Run(c.name, func(t *testing.T) {
			resp, err := c.a.Lease(c.req)
			if c.want && err != nil {
				t.Errorf("Lease() unexpectedly failed: %v", err)
			} else if !c.want && err == nil {
				t.Errorf("Lease() unexpectedly succeeded: %v", resp)
			}
		})
	}
}

func TestListener(t *testing.T) {
	ch := make(chan *gpupb.LeaseResponse, 1)
	a := New(O{
		AmbientTraffic: ch,
		LocalAllocator: local.New([]*gpupb.GPU{
			&gpupb.GPU{
				Id: 100,
			},
		}, 0),
	}, 0)

	// Emulate a fulfillment request from some other node.
	ch <- &gpupb.LeaseResponse{
		Requestor: "some-originator",
		Lease: &gpupb.Lease{
			Token:      "some-token",
			Expiration: tpb.New(time.Now().Add(time.Hour)),
		},
	}

	time.Sleep(time.Second)

	resp, err := a.Lease(&gpupb.LeaseRequest{
		Requestor: "some-originator",
		Token:     "some-token",
		Duration:  dpb.New(time.Hour),
	})

	if err == nil {
		t.Errorf("Lease() unexpectedly succeeded: %v", resp)
	}
}
