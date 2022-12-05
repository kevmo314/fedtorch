package p2p

import (
	"fmt"
	"time"

	"github.com/nictuku/dht"
)

type Topic int

const (
	TopicUnknown Topic = iota
	TopicHasGPUCapacity
)

var (
	Timeout = 30 * time.Second

	Topics = map[Topic]dht.InfoHash{
		TopicHasGPUCapacity: "0xdeadbeef",
	}
)

type Peer string

type Store struct {
	host string
	dht  *dht.DHT
}

type O struct {
	// Address is the governor address.
	Address string

	// Port is the governor port.
	Port int
}

func New(o O) *Store {
	c := dht.NewConfig()
	c.Address = o.Address
	c.Port = o.Port

	t, err := dht.New(c)
	if err != nil {
		panic(fmt.Sprintf("could not create a new DHT instance: %v", err))
	}

	return &Store{
		dht:  t,
		host: fmt.Sprintf("%v:%v", o.Address, t.Port()),
	}
}

func (s *Store) Start() error { return s.dht.Start() }
func (s *Store) Stop()        { s.dht.Stop() }
func (s *Store) Host() string { return s.host }

func (s *Store) Announce(t Topic) { s.dht.PeersRequest(string(Topics[t]), true) }
func (s *Store) Revoke(t Topic)   { s.dht.RemoveInfoHash(string(Topics[t])) }

func (s *Store) Query(t Topic, max int) []Peer {
	s.dht.PeersRequest(string(Topics[t]), false)

	var peers []Peer

	select {
	case d := <-s.dht.PeersRequestResults:
		for et, eps := range d {
			if et == Topics[t] {
				for _, ep := range eps {
					peers = append(peers, Peer(dht.DecodePeerAddress(ep)))
					if len(peers) >= max {
						return peers
					}
				}
			}

		}
	case <-time.After(Timeout):
	}
	return peers
}
