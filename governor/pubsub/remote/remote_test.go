package remote

import (
	"testing"
	"time"

	"github.com/kevmo314/fedtorch/governor/pubsub/local"

	gpupb "github.com/kevmo314/fedtorch/governor/api/go/gpu"
	dpb "google.golang.org/protobuf/types/known/durationpb"
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
