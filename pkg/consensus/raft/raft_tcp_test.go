package raftcons

import (
    "context"
    "encoding/json"
    "testing"
    "time"

    "github.com/hashicorp/raft"
)

// Three-node election using real TCP transports and on-disk stores (in temp dirs).
func TestRaft_ThreeNodeElection_TCP(t *testing.T) {
    t.Parallel()

    mk := func(id string) *Node {
        n, err := New(Options{
            NodeID:           id,
            BindAddr:         "127.0.0.1:0",
            DataDir:          t.TempDir(),
            SnapshotsRetained: 1,
            HeartbeatTimeout:  150 * time.Millisecond,
            ElectionTimeout:   300 * time.Millisecond,
            CommitTimeout:     50 * time.Millisecond,
            ApplyTimeout:      2 * time.Second,
        })
        if err != nil { t.Fatalf("new %s: %v", id, err) }
        return n
    }

    n1 := mk("n1"); n1.opts.Bootstrap = true
    n2 := mk("n2")
    n3 := mk("n3")

    ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
    defer cancel()

    for _, n := range []*Node{n1, n2, n3} {
        if err := n.Start(ctx); err != nil { t.Fatalf("start %s: %v", n.opts.NodeID, err) }
        defer n.Stop()
    }

    // Wait for n1 to become leader
    deadline := time.Now().Add(5 * time.Second)
    for time.Now().Before(deadline) {
        if n1.IsLeader() { break }
        time.Sleep(50 * time.Millisecond)
    }
    if !n1.IsLeader() { t.Fatalf("n1 did not become leader") }

    // Add voters
    add := func(id string, addr string) {
        f := n1.r.AddVoter(raft.ServerID(id), raft.ServerAddress(addr), 0, 3*time.Second)
        if err := f.Error(); err != nil { t.Fatalf("AddVoter %s: %v", id, err) }
    }
    add("n2", string(n2.addr))
    add("n3", string(n3.addr))

    // All nodes should know the leader
    awaitLeaderKnown := func(n *Node) {
        t.Helper()
        dl := time.Now().Add(5 * time.Second)
        for time.Now().Before(dl) {
            if id, _, ok := n.Leader(); ok && id != "" { return }
            time.Sleep(50 * time.Millisecond)
        }
        t.Fatalf("leader unknown on %s", n.opts.NodeID)
    }
    awaitLeaderKnown(n1)
    awaitLeaderKnown(n2)
    awaitLeaderKnown(n3)

    // Apply a membership command on leader and ensure replication
    type addReq struct{ ID string `json:"id"`; Addr string `json:"addr"` }
    // Apply AddNode for a logical service member "svc-1"
    payload := mustJSON(t, struct{ ID, Addr string }{ID: "svc-1", Addr: "10.0.0.1:9999"})
    // Use raft Apply via consensus wrapper
    if err := n1.Apply(cCommand{Op: "AddNode", Payload: payload}, 2*time.Second); err != nil {
        t.Fatalf("apply add: %v", err)
    }

    // Verify propagation by inspecting snapshots
    awaitHasMember := func(n *Node, id string) {
        dl := time.Now().Add(5 * time.Second)
        for time.Now().Before(dl) {
            snap, err := n.StateSnapshot()
            if err == nil && snapshotHasID(snap, id) {
                return
            }
            time.Sleep(50 * time.Millisecond)
        }
        t.Fatalf("state did not include %s on %s", id, n.opts.NodeID)
    }
    awaitHasMember(n1, "svc-1")
    awaitHasMember(n2, "svc-1")
    awaitHasMember(n3, "svc-1")

    // Now remove the member and ensure it's gone from all nodes
    rmPayload := mustJSON(t, struct{ ID string `json:"id"` }{ID: "svc-1"})
    if err := n1.Apply(cCommand{Op: "RemoveNode", Payload: rmPayload}, 2*time.Second); err != nil {
        t.Fatalf("apply remove: %v", err)
    }
    awaitNoMember := func(n *Node, id string) {
        dl := time.Now().Add(5 * time.Second)
        for time.Now().Before(dl) {
            snap, err := n.StateSnapshot()
            if err == nil && !snapshotHasID(snap, id) {
                return
            }
            time.Sleep(50 * time.Millisecond)
        }
        t.Fatalf("state still includes %s on %s", id, n.opts.NodeID)
    }
    awaitNoMember(n1, "svc-1")
    awaitNoMember(n2, "svc-1")
    awaitNoMember(n3, "svc-1")
}

// Helpers pulling in consensus.Command without import dance
type cCommand = struct{ Op string; Payload []byte }

func mustJSON(t *testing.T, v interface{}) []byte {
    t.Helper()
    b, err := json.Marshal(v)
    if err != nil { t.Fatalf("json: %v", err) }
    return b
}

func snapshotHasID(b []byte, id string) bool {
    // Snapshot schema: {"version":1,"members":[{"id":"..."},...]}
    var s struct{ Members []struct{ ID string `json:"id"` } `json:"members"` }
    if err := json.Unmarshal(b, &s); err != nil { return false }
    for _, m := range s.Members { if m.ID == id { return true } }
    return false
}
// no extra helpers
