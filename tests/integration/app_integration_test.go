//go:build integration

package integration

import (
    "context"
    "sync/atomic"
    "testing"
    "time"

    "github.com/amirimatin/go-cluster/pkg/bootstrap"
    httpjson "github.com/amirimatin/go-cluster/pkg/transport/httpjson"
    "github.com/amirimatin/go-cluster/pkg/transport"
    gcluster "github.com/amirimatin/go-cluster/pkg/cluster"
)

type testHandlers struct {
    nodeID    string
    writes    atomic.Int64
    syncs     atomic.Int64
}

func (h *testHandlers) HandleWrite(ctx context.Context, op string, req []byte) ([]byte, error) {
    h.writes.Add(1)
    return []byte("leader=" + h.nodeID + " op=" + op + " req=" + string(req)), nil
}
func (h *testHandlers) HandleRead(ctx context.Context, op string, req []byte) ([]byte, error)  { return nil, nil }
func (h *testHandlers) HandleSync(ctx context.Context, topic string, data []byte) error        { h.syncs.Add(1); return nil }

func TestAppWrite_ForwardToLeader(t *testing.T) {
    ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
    defer cancel()

    h1, h2, h3 := &testHandlers{nodeID: "n1"}, &testHandlers{nodeID: "n2"}, &testHandlers{nodeID: "n3"}
    n1, n2, n3 := mustStartThreeNodesWithHandlers(t, ctx, h1, h2, h3)
    defer n3.Close(); defer n2.Close(); defer n1.Close()

    cli := httpjson.NewClient(3 * time.Second)
    // Wait until leader n1 is ready
    waitUntil(t, 20*time.Second, func() error {
        s, err := fetchStatus(ctx, cli, "127.0.0.1:17976")
        if err != nil { return err }
        if !s.Healthy || s.LeaderID != "n1" { return errNotYet }
        return nil
    })

    // Join voters via leader mgmt (n1)
    joinCtx, cancelJoin := context.WithTimeout(ctx, 5*time.Second)
    if _, err := cli.PostJoin(joinCtx, "127.0.0.1:17976", transport.JoinRequest{ID: "n2", RaftAddr: "127.0.0.1:9552"}); err != nil { cancelJoin(); t.Fatalf("join n2: %v", err) }
    if _, err := cli.PostJoin(joinCtx, "127.0.0.1:17976", transport.JoinRequest{ID: "n3", RaftAddr: "127.0.0.1:9553"}); err != nil { cancelJoin(); t.Fatalf("join n3: %v", err) }
    cancelJoin()

    var out []byte
    waitUntil(t, 20*time.Second, func() error {
        var err error
        out, err = n2.AppWrite(ctx, "op1", []byte("hello"))
        if err != nil { return errNotYet }
        return nil
    })
    if len(out) == 0 { t.Fatalf("appwrite empty response") }

    time.Sleep(200 * time.Millisecond)
    if h1.writes.Load() != 1 { t.Fatalf("leader writes=%d want=1", h1.writes.Load()) }
    if h2.writes.Load() != 0 || h3.writes.Load() != 0 { t.Fatalf("followers wrote: n2=%d n3=%d", h2.writes.Load(), h3.writes.Load()) }
}

func mustStartThreeNodesWithHandlers(t *testing.T, ctx context.Context, h1, h2, h3 *testHandlers) (c1, c2, c3 *gcluster.Cluster) {
    t.Helper()
    n1, err := bootstrap.Run(ctx, bootstrap.Config{NodeID: "n1", RaftAddr: "127.0.0.1:9551", MemBind: "127.0.0.1:7976", MgmtAddr: "127.0.0.1:17976", DiscoveryKind: "static", SeedsCSV: "", Bootstrap: true, AppHandlers: h1})
    if err != nil { t.Fatalf("n1: %v", err) }
    n2, err := bootstrap.Run(ctx, bootstrap.Config{NodeID: "n2", RaftAddr: "127.0.0.1:9552", MemBind: "127.0.0.1:8976", MgmtAddr: "127.0.0.1:18976", DiscoveryKind: "static", SeedsCSV: "127.0.0.1:7976", AppHandlers: h2})
    if err != nil { t.Fatalf("n2: %v", err) }
    n3, err := bootstrap.Run(ctx, bootstrap.Config{NodeID: "n3", RaftAddr: "127.0.0.1:9553", MemBind: "127.0.0.1:9976", MgmtAddr: "127.0.0.1:19976", DiscoveryKind: "static", SeedsCSV: "127.0.0.1:7976", AppHandlers: h3})
    if err != nil { t.Fatalf("n3: %v", err) }
    return n1, n2, n3
}
