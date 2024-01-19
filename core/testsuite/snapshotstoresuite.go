package testsuite

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/hallgren/eventsourcing/core"
)

type snapshotstoreFunc = func() (core.SnapshotStore, func(), error)

func TestSnapshotStore(t *testing.T, ssFunc snapshotstoreFunc) {
	tests := []struct {
		title string
		run   func(es core.SnapshotStore) error
	}{
		{"should save and get snapshot", saveAndGetSnapshot},
		{"should get error when getting none existing snapshot", getNoneExistingSnapshot},
	}

	for _, test := range tests {
		t.Run(test.title, func(t *testing.T) {
			ss, closeFunc, err := ssFunc()
			if err != nil {
				t.Fatal(err)
			}
			err = test.run(ss)
			if err != nil {
				// make use of t.Error instead of t.Fatal to make sure the closeFunc is executed
				t.Error(err)
			}
			closeFunc()
		})
	}
}

func saveAndGetSnapshot(ss core.SnapshotStore) error {
	snapshot := core.Snapshot{
		ID:            "id",
		Type:          "person",
		Version:       1,
		GlobalVersion: 1,
		State:         []byte("123"),
	}

	err := ss.Save(snapshot)
	if err != nil {
		return err
	}

	s, err := ss.Get(context.Background(), "id", "person")
	if err != nil {
		return err
	}

	if string(s.State) != "123" {
		return errors.New("wrong snapshot state")
	}

	if s.Version != snapshot.Version {
		return fmt.Errorf("exp version %d got %d", snapshot.Version, s.Version)
	}

	if s.GlobalVersion != snapshot.GlobalVersion {
		return fmt.Errorf("exp global version %d got %d", snapshot.GlobalVersion, s.GlobalVersion)
	}

	s, err = ss.Get(context.Background(), "none_existing_id", "person")
	if !errors.Is(err, core.ErrSnapshotNotFound) {
		return err
	}
	return nil
}

func getNoneExistingSnapshot(ss core.SnapshotStore) error {
	_, err := ss.Get(context.Background(), "id", "person")
	if !errors.Is(err, core.ErrSnapshotNotFound) {
		return err
	}
	return nil
}
