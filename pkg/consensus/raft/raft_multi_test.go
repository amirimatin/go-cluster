package raftcons

import (
    "context"
    "testing"
    "time"

    "github.com/hashicorp/raft"
)

// This test wires three Raft nodes using in-memory loopback transports
// to validate leader election and that all nodes learn the leader.
func TestRaft_ThreeNodeElection_Inmem(t *testing.T) {
    // Build nodes
    n1, _ := New(Options{NodeID: "n1", Bootstrap: true, ApplyTimeout: 2 * time.Second})
    n2, _ := New(Options{NodeID: "n2"})
    n3, _ := New(Options{NodeID: "n3"})

    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    // Start nodes
    if err := n1.Start(ctx); err != nil { t.Fatalf("n1 start: %v", err) }
    if err := n2.Start(ctx); err != nil { t.Fatalf("n2 start: %v", err) }
    if err := n3.Start(ctx); err != nil { t.Fatalf("n3 start: %v", err) }
    defer n1.Stop(); defer n2.Stop(); defer n3.Stop()

    // Fully connect transports pairwise (loopback)
    connect := func(a, b *Node) {
        if a.lb == nil || b.lb == nil { t.Fatalf("loopback transport expected") }
        a.lb.Connect(b.addr, b.trans)
        b.lb.Connect(a.addr, a.trans)
    }
    connect(n1, n2)
    connect(n1, n3)
    connect(n2, n3)

    // Wait until n1 is leader as it's bootstrapped
    deadline := time.Now().Add(3 * time.Second)
    for time.Now().Before(deadline) {
        if n1.IsLeader() { break }
        time.Sleep(50 * time.Millisecond)
    }
    if !n1.IsLeader() { t.Fatalf("n1 did not become leader") }

    // Add n2 and n3 as voters to n1's cluster
    add := func(id string, addr raft.ServerAddress) {
        f := n1.r.AddVoter(raft.ServerID(id), addr, 0, 2*time.Second)
        if err := f.Error(); err != nil { t.Fatalf("AddVoter %s: %v", id, err) }
    }
    add("n2", n2.addr)
    add("n3", n3.addr)

    // All nodes should agree on a leader eventually
    awaitLeaderKnown := func(n *Node) {
        t.Helper()
        deadline := time.Now().Add(5 * time.Second)
        for time.Now().Before(deadline) {
            id, _, ok := n.Leader()
            if ok && id != "" { return }
            time.Sleep(50 * time.Millisecond)
        }
        t.Fatalf("leader unknown on node %v", n.opts.NodeID)
    }
    awaitLeaderKnown(n1)
    awaitLeaderKnown(n2)
    awaitLeaderKnown(n3)
}

