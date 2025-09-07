//go:build integration

package integration

import (
    "context"
    "encoding/json"
    "testing"
    "time"

    "github.com/amirimatin/go-cluster/pkg/bootstrap"
    httpjson "github.com/amirimatin/go-cluster/pkg/transport/httpjson"
)

func TestFollowerStatus_ProxiesToLeader(t *testing.T) {
    ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
    defer cancel()

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

    cli := httpjson.NewClient(3 * time.Second)

    waitUntil(t, 10*time.Second, func() error {
        s, err := fetchStatus2(ctx, cli, "127.0.0.1:17946")
        if err != nil { return err }
        if !s.Healthy || s.LeaderID != "n1" { return errNotYet }
        return nil
    })

    waitUntil(t, 10*time.Second, func() error {
        s2, err := fetchStatus2(ctx, cli, "127.0.0.1:18946")
        if err != nil { return err }
        if s2.LeaderID != "n1" || s2.LeaderAddr != "127.0.0.1:17946" { return errNotYet }
        return nil
    })
}

type pstatus struct {
    Healthy    bool   `json:"Healthy"`
    Term       uint64 `json:"Term"`
    LeaderID   string `json:"LeaderID"`
    LeaderAddr string `json:"LeaderAddr"`
}

func fetchStatus2(ctx context.Context, cli *httpjson.Client, addr string) (pstatus, error) {
    var s pstatus
    b, err := cli.GetStatus(ctx, addr)
    if err != nil { return s, err }
    if err := json.Unmarshal(b, &s); err != nil { return s, err }
    return s, nil
}

