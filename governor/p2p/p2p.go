package p2p

import (
	"time"

	"github.com/nictuku/dht"
)

type Topic int

const (
	TopicUnknown Topic = iota
	TopicHasCapacity
)

var (
	Timeout = 30 * time.Second

	Topics = map[Topic]dht.InfoHash{
		TopicHasCapacity: "0xdeadbeef",
	}
)

type Peer string

type Store struct {
	dht *dht.DHT
}

func (s Store) Announce(t Topic) { s.dht.PeersRequest(string(Topics[t]), true) }
func (s Store) Revoke(t Topic)   { s.dht.RemoveInfoHash(string(Topics[t])) }

func (s Store) Query(t Topic, max int) []Peer {
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
