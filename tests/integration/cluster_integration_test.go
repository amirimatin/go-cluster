//go:build integration

package integration

import (
    "context"
    "encoding/json"
    "testing"
    "time"

    "github.com/amirimatin/go-cluster/pkg/bootstrap"
    gcluster "github.com/amirimatin/go-cluster/pkg/cluster"
    httpjson "github.com/amirimatin/go-cluster/pkg/transport/httpjson"
    "github.com/amirimatin/go-cluster/pkg/transport"
)

type status struct {
    Healthy    bool   `json:"Healthy"`
    Term       uint64 `json:"Term"`
    LeaderID   string `json:"LeaderID"`
    LeaderAddr string `json:"LeaderAddr"`
    Members    []struct{ ID string `json:"ID"` } `json:"Members"`
}

func TestLeaderChange_OnLeaderStopElectNewLeader(t *testing.T) {
    ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
    defer cancel()

    n1, n2, n3 := mustStartThreeNodes(t, ctx)
    defer n3.Close()
    defer n2.Close()
    defer n1.Close()

    // Join voters via leader mgmt (n1)
    cli := httpjson.NewClient(3 * time.Second)
    joinCtx, cancelJoin := context.WithTimeout(ctx, 5*time.Second)
    if _, err := cli.PostJoin(joinCtx, "127.0.0.1:17946", transport.JoinRequest{ID: "n2", RaftAddr: "127.0.0.1:9522"}); err != nil {
        cancelJoin(); t.Fatalf("join n2: %v", err)
    }
    if _, err := cli.PostJoin(joinCtx, "127.0.0.1:17946", transport.JoinRequest{ID: "n3", RaftAddr: "127.0.0.1:9523"}); err != nil {
        cancelJoin(); t.Fatalf("join n3: %v", err)
    }
    cancelJoin()

    // Wait for n1 to be recognized as leader
    waitUntil(t, 10*time.Second, func() error {
        s, err := fetchStatus(ctx, cli, "127.0.0.1:17946")
        if err != nil { return err }
        if !s.Healthy || s.LeaderID != "n1" { return errNotYet }
        return nil
    })

    // Stop leader n1 to force re-election
    _ = n1.Close()

    // New leader should be n2 or n3
    waitUntil(t, 15*time.Second, func() error {
        s2, err := fetchStatus(ctx, cli, "127.0.0.1:18946")
        if err != nil { return err }
        if s2.LeaderID != "n2" && s2.LeaderID != "n3" { return errNotYet }
        return nil
    })
}

func TestLeave_RemovesNodeAndConverges(t *testing.T) {
    ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
    defer cancel()

    n1, n2, n3 := mustStartThreeNodes(t, ctx)
    defer n2.Close()
    defer n1.Close()

    cli := httpjson.NewClient(3 * time.Second)
    // Join voters via leader mgmt (n1)
    joinCtx, cancelJoin := context.WithTimeout(ctx, 5*time.Second)
    if _, err := cli.PostJoin(joinCtx, "127.0.0.1:17946", transport.JoinRequest{ID: "n2", RaftAddr: "127.0.0.1:9522"}); err != nil {
        cancelJoin(); t.Fatalf("join n2: %v", err)
    }
    if _, err := cli.PostJoin(joinCtx, "127.0.0.1:17946", transport.JoinRequest{ID: "n3", RaftAddr: "127.0.0.1:9523"}); err != nil {
        cancelJoin(); t.Fatalf("join n3: %v", err)
    }
    cancelJoin()

    // Ensure cluster has a leader (n1) before leave
    waitUntil(t, 10*time.Second, func() error {
        s, err := fetchStatus(ctx, cli, "127.0.0.1:17946")
        if err != nil { return err }
        if !s.Healthy || s.LeaderID != "n1" { return errNotYet }
        return nil
    })

    // Request removal of n3 from leader, then stop n3 to propagate membership leave
    leaveCtx, cancelLeave := context.WithTimeout(ctx, 5*time.Second)
    if _, err := cli.PostLeave(leaveCtx, "127.0.0.1:17946", transport.LeaveRequest{ID: "n3"}); err != nil {
        cancelLeave(); t.Fatalf("leave n3: %v", err)
    }
    cancelLeave()
    _ = n3.Close()

    // Poll n1 status until only two members remain and n3 is absent
    waitUntil(t, 20*time.Second, func() error {
        s, err := fetchStatus(ctx, cli, "127.0.0.1:17946")
        if err != nil { return err }
        if len(s.Members) != 2 { return errNotYet }
        var hasN3 bool
        for _, m := range s.Members { if m.ID == "n3" { hasN3 = true; break } }
        if hasN3 { return errNotYet }
        return nil
    })
}

// Helpers

func mustStartThreeNodes(t *testing.T, ctx context.Context) (n1, n2, n3 *gcluster.Cluster) {
    t.Helper()
    c1, err := bootstrap.Run(ctx, bootstrap.Config{
        NodeID:        "n1",
        RaftAddr:      "127.0.0.1:9521",
        MemBind:       "127.0.0.1:7946",
        MgmtAddr:      "127.0.0.1:17946",
        DiscoveryKind: "static",
        SeedsCSV:      "",
        Bootstrap:     true,
    })
    if err != nil { t.Fatalf("n1: %v", err) }

    c2, err := bootstrap.Run(ctx, bootstrap.Config{
        NodeID:        "n2",
        RaftAddr:      "127.0.0.1:9522",
        MemBind:       "127.0.0.1:8946",
        MgmtAddr:      "127.0.0.1:18946",
        DiscoveryKind: "static",
        SeedsCSV:      "127.0.0.1:7946",
    })
    if err != nil { t.Fatalf("n2: %v", err) }

    c3, err := bootstrap.Run(ctx, bootstrap.Config{
        NodeID:        "n3",
        RaftAddr:      "127.0.0.1:9523",
        MemBind:       "127.0.0.1:9946",
        MgmtAddr:      "127.0.0.1:19946",
        DiscoveryKind: "static",
        SeedsCSV:      "127.0.0.1:7946",
    })
    if err != nil { t.Fatalf("n3: %v", err) }
    return c1, c2, c3
}

var errNotYet = &temporaryError{}
type temporaryError struct{}
func (e *temporaryError) Error() string { return "not yet" }

func waitUntil(t *testing.T, timeout time.Duration, fn func() error) {
    t.Helper()
    deadline := time.Now().Add(timeout)
    var last error
    for time.Now().Before(deadline) {
        if err := fn(); err == nil {
            return
        } else {
            last = err
        }
        time.Sleep(200 * time.Millisecond)
    }
    t.Fatalf("timeout waiting for condition: %v", last)
}

func fetchStatus(ctx context.Context, cli *httpjson.Client, addr string) (status, error) {
    var s status
    b, err := cli.GetStatus(ctx, addr)
    if err != nil { return s, err }
    if err := json.Unmarshal(b, &s); err != nil { return s, err }
    return s, nil
}

