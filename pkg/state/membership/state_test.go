package membership

import (
    "testing"
    m "github.com/amirimatin/go-cluster/pkg/membership"
)

func TestState_AddRemoveSnapshotRestore(t *testing.T) {
    s := New()

    n1 := m.MemberInfo{ID: "n1", Addr: "127.0.0.1:1001", Meta: map[string]string{"ver":"0.1.0"}}
    n2 := m.MemberInfo{ID: "n2", Addr: "127.0.0.1:1002"}

    if err := s.ApplyAddNode(n1); err != nil {
        t.Fatalf("add n1: %v", err)
    }
    if err := s.ApplyAddNode(n2); err != nil {
        t.Fatalf("add n2: %v", err)
    }

    snap, err := s.Snapshot()
    if err != nil {
        t.Fatalf("snapshot: %v", err)
    }
    if len(snap) == 0 { t.Fatalf("empty snapshot") }

    // Remove one and snapshot again
    if err := s.ApplyRemoveNode("n1"); err != nil {
        t.Fatalf("remove n1: %v", err)
    }

    // Restore from the first snapshot and ensure n1 returns.
    s2 := New()
    if err := s2.Restore(snap); err != nil {
        t.Fatalf("restore: %v", err)
    }
    // Take a new snapshot of restored state to verify it round-trips.
    snap2, err := s2.Snapshot()
    if err != nil {
        t.Fatalf("snapshot2: %v", err)
    }
    if string(snap2) != string(snap) {
        t.Fatalf("round-trip mismatch:\n got: %s\nwant: %s", string(snap2), string(snap))
    }
}

func TestState_ErrorsOnEmptyID(t *testing.T) {
    s := New()
    if err := s.ApplyAddNode(m.MemberInfo{}); err == nil {
        t.Fatalf("expected error on empty id")
    }
    if err := s.ApplyRemoveNode(""); err == nil {
        t.Fatalf("expected error on empty id for remove")
    }
}

