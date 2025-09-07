package raftcons

import (
    "context"
    "testing"
    "time"
)

func TestRaft_SingleNodeLeadership(t *testing.T) {
    n, err := New(Options{NodeID: "n1", Bootstrap: true, ApplyTimeout: 2 * time.Second})
    if err != nil { t.Fatalf("new: %v", err) }

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    if err := n.Start(ctx); err != nil { t.Fatalf("start: %v", err) }
    defer n.Stop()

    // Wait until IsLeader becomes true or times out
    deadline := time.Now().Add(3 * time.Second)
    for time.Now().Before(deadline) {
        if n.IsLeader() { break }
        time.Sleep(50 * time.Millisecond)
    }
    if !n.IsLeader() { t.Fatalf("node did not become leader in time") }

    // Ensure we receive a leadership notification
    select {
    case li, ok := <-n.LeaderCh():
        if !ok { t.Fatalf("leader channel closed unexpectedly") }
        if li.ID != "n1" { t.Fatalf("leader id = %q, want n1", li.ID) }
    case <-time.After(2 * time.Second):
        t.Fatalf("timed out waiting for leader event")
    }
}

