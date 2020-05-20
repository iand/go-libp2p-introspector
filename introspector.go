package introspector

import (
	"fmt"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p-core/introspection"
	introspection_pb "github.com/libp2p/go-libp2p-core/introspection/pb"

	"github.com/imdario/mergo"
)

var _ introspection.Introspector = (*DefaultIntrospector)(nil)

// DefaultIntrospector is a registry of subsystem data/metrics providers and also allows
// clients to inspect the system state by calling all the providers registered with it
type DefaultIntrospector struct {
	treeMu sync.RWMutex
	tree   *introspection.DataProviders

	snapshotStartTime time.Time
}

func NewDefaultIntrospector() *DefaultIntrospector {
	return &DefaultIntrospector{tree: &introspection.DataProviders{}, snapshotStartTime: time.Now()}
}

func (d *DefaultIntrospector) RegisterDataProviders(provs *introspection.DataProviders) error {
	d.treeMu.Lock()
	defer d.treeMu.Unlock()

	if err := mergo.Merge(d.tree, provs); err != nil {
		return err
	}

	return nil
}

func (d *DefaultIntrospector) FetchRuntime() (*introspection_pb.Runtime, error) {
	var err error
	r := &introspection_pb.Runtime{}
	if d.tree.Runtime != nil {
		if r, err = d.tree.Runtime(); err != nil {
			return nil, fmt.Errorf("failed to fetch runtime info, err=%s", err)
		}
	}
	return r, err
}

func (d *DefaultIntrospector) FetchFullState() (*introspection_pb.State, error) {
	d.treeMu.RLock()
	defer d.treeMu.RUnlock()

	s := &introspection_pb.State{}

	// timestamps
	s.StartTs = timeToUnixMillis(d.snapshotStartTime)
	s.InstantTs = timeToUnixMillis(time.Now())
	d.snapshotStartTime = time.Now()

	// subsystems
	s.Subsystems = &introspection_pb.Subsystems{}

	// connections
	if d.tree.Connection != nil {
		conns, err := d.tree.Connection(introspection.ConnectionQueryParams{Output: introspection.QueryOutputFull})
		if err != nil {
			return nil, fmt.Errorf("failed to fetch connections: %wsvc", err)
		}
		// resolve streams on connection
		if d.tree.Stream != nil {
			for _, c := range conns {
				var sids []introspection.StreamID
				for _, s := range c.Streams.StreamIds {
					sids = append(sids, introspection.StreamID(s))
				}

				sl, err := d.tree.Stream(introspection.StreamQueryParams{Output: introspection.QueryOutputFull, Include: sids})
				if err != nil {
					return nil, fmt.Errorf("failed to fetch streams for connection: %wsvc", err)
				}
				c.Streams = sl
			}
		}
		s.Subsystems.Connections = conns
	}

	// traffic
	if d.tree.Traffic != nil {
		tr, err := d.tree.Traffic()
		if err != nil {
			return nil, fmt.Errorf("failed to fetch traffic: %wsvc", err)
		}
		s.Traffic = tr
	}

	return s, nil
}

func timeToUnixMillis(t time.Time) uint64 {
	return uint64(t.UnixNano() / 1000000)
}
