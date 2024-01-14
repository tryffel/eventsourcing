package memory_test

import (
	"context"
	"errors"
	"testing"

	"github.com/hallgren/eventsourcing/core"
	"github.com/hallgren/eventsourcing/snapshotstore/memory"
)

func TestSaveAndGetSnapshot(t *testing.T) {
	store := memory.Create()

	snapshot := core.Snapshot{
		ID:            "id",
		Type:          "person",
		Version:       1,
		GlobalVersion: 1,
		State:         []byte("123"),
	}

	err := store.Save(snapshot)
	if err != nil {
		t.Fatal(err)
	}

	s, err := store.Get(context.Background(), "id", "person")
	if err != nil {
		t.Fatal(err)
	}

	if string(s.State) != "123" {
		t.Fatal("wrong snapshot state")
	}

	if s.Version != snapshot.Version {
		t.Fatalf("exp version %d got %d", snapshot.Version, s.Version)
	}

	if s.GlobalVersion != snapshot.GlobalVersion {
		t.Fatalf("exp global version %d got %d", snapshot.GlobalVersion, s.GlobalVersion)
	}

	s, err = store.Get(context.Background(), "none_existing_id", "person")
	if !errors.Is(err, core.ErrSnapshotNotFound) {
		t.Fatal(err)
	}
}
