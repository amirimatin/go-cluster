//go:build integration

package integration

import (
    "context"
    "testing"
    "time"

    "github.com/amirimatin/go-cluster/pkg/bootstrap"
    httpjson "github.com/amirimatin/go-cluster/pkg/transport/httpjson"
    "github.com/amirimatin/go-cluster/pkg/transport"
)

func TestThreeNodes_JoinAndStatus(t *testing.T) {
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    // Start leader (bootstrap)
    n1, err := bootstrap.Run(ctx, bootstrap.Config{
        NodeID:        "n1",
        RaftAddr:      "127.0.0.1:9521",
        MemBind:       "127.0.0.1:7946",
        MgmtAddr:      "127.0.0.1:17946",
        DiscoveryKind: "static",
        SeedsCSV:      "",
        Bootstrap:     true,
    })
    if err != nil { t.Fatalf("n1: %v", err) }
    defer n1.Close()

    // Start n2/n3 (membership join to n1)
    n2, err := bootstrap.Run(ctx, bootstrap.Config{
        NodeID:        "n2",
        RaftAddr:      "127.0.0.1:9522",
        MemBind:       "127.0.0.1:8946",
        MgmtAddr:      "127.0.0.1:18946",
        DiscoveryKind: "static",
        SeedsCSV:      "127.0.0.1:7946",
    })
    if err != nil { t.Fatalf("n2: %v", err) }
    defer n2.Close()

    n3, err := bootstrap.Run(ctx, bootstrap.Config{
        NodeID:        "n3",
        RaftAddr:      "127.0.0.1:9523",
        MemBind:       "127.0.0.1:9946",
        MgmtAddr:      "127.0.0.1:19946",
        DiscoveryKind: "static",
        SeedsCSV:      "127.0.0.1:7946",
    })
    if err != nil { t.Fatalf("n3: %v", err) }
    defer n3.Close()

    // Join RAFT via leader management
    cli := httpjson.NewClient(3 * time.Second)
    joinCtx, cancelJoin := context.WithTimeout(ctx, 5*time.Second)
    defer cancelJoin()
    if _, err := cli.PostJoin(joinCtx, "127.0.0.1:17946", transport.JoinRequest{ID: "n2", RaftAddr: "127.0.0.1:9522"}); err != nil {
        t.Fatalf("join n2: %v", err)
    }
    if _, err := cli.PostJoin(joinCtx, "127.0.0.1:17946", transport.JoinRequest{ID: "n3", RaftAddr: "127.0.0.1:9523"}); err != nil {
        t.Fatalf("join n3: %v", err)
    }

    // Poll leader status until healthy and leader reported
    deadline := time.Now().Add(10 * time.Second)
    for {
        s, err := fetchStatus(ctx, cli, "127.0.0.1:17946")
        if err == nil && s.Healthy && s.LeaderID == "n1" {
            break
        }
        if time.Now().After(deadline) {
            t.Fatalf("status not healthy/leader: %+v, err=%v", s, err)
        }
        time.Sleep(200 * time.Millisecond)
    }
}

