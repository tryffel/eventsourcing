package memory

import (
	"context"

	"github.com/hallgren/eventsourcing/core"
)

type Memory struct {
	snapshots map[string]core.Snapshot
}

// Create in memory snapshot store
func Create() *Memory {
	return &Memory{
		snapshots: make(map[string]core.Snapshot),
	}
}

func (m *Memory) Get(ctx context.Context, aggregateID, aggregateType string) (core.Snapshot, error) {
	snapshot, ok := m.snapshots[aggregateType+"_"+aggregateID]
	if !ok {
		return core.Snapshot{}, core.ErrSnapshotNotFound
	}
	return snapshot, nil
}

func (m *Memory) Save(snapshot core.Snapshot) error {
	m.snapshots[snapshot.Type+"_"+snapshot.ID] = snapshot
	return nil
}
