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

// Simulate a short disconnection of a follower (n3) and verify rejoin converges back to 3 members.
func TestTemporaryDisconnect_RejoinConverges(t *testing.T) {
    ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
    defer cancel()

    n1, n2, n3 := mustStartThreeNodes(t, ctx)
    defer n2.Close()
    defer n1.Close()

    cli := httpjson.NewClient(3 * time.Second)

    // Join voters via leader mgmt (n1)
    joinCtx, cancelJoin := context.WithTimeout(ctx, 5*time.Second)
    if _, err := cli.PostJoin(joinCtx, "127.0.0.1:17946", transport.JoinRequest{ID: "n2", RaftAddr: "127.0.0.1:9522"}); err != nil { cancelJoin(); t.Fatalf("join n2: %v", err) }
    if _, err := cli.PostJoin(joinCtx, "127.0.0.1:17946", transport.JoinRequest{ID: "n3", RaftAddr: "127.0.0.1:9523"}); err != nil { cancelJoin(); t.Fatalf("join n3: %v", err) }
    cancelJoin()

    waitUntil(t, 10*time.Second, func() error {
        s, err := fetchStatus(ctx, cli, "127.0.0.1:17946")
        if err != nil { return err }
        if !s.Healthy || s.LeaderID != "n1" { return errNotYet }
        return nil
    })

    // Simulate short disconnection: stop n3
    _ = n3.Close()

    waitUntil(t, 20*time.Second, func() error {
        s, err := fetchStatus(ctx, cli, "127.0.0.1:17946")
        if err != nil { return err }
        if len(s.Members) != 2 { return errNotYet }
        return nil
    })

    // Start n3 again and rejoin
    n3b, err := bootstrap.Run(ctx, bootstrap.Config{
        NodeID:        "n3",
        RaftAddr:      "127.0.0.1:9523",
        MemBind:       "127.0.0.1:9946",
        MgmtAddr:      "127.0.0.1:19946",
        DiscoveryKind: "static",
        SeedsCSV:      "127.0.0.1:7946",
    })
    if err != nil { t.Fatalf("n3 restart: %v", err) }
    defer n3b.Close()

    rejoinCtx, cancelRejoin := context.WithTimeout(ctx, 5*time.Second)
    if _, err := cli.PostJoin(rejoinCtx, "127.0.0.1:17946", transport.JoinRequest{ID: "n3", RaftAddr: "127.0.0.1:9523"}); err != nil { cancelRejoin(); t.Fatalf("rejoin n3: %v", err) }
    cancelRejoin()

    waitUntil(t, 20*time.Second, func() error {
        s, err := fetchStatus(ctx, cli, "127.0.0.1:17946")
        if err != nil { return err }
        if len(s.Members) != 3 { return errNotYet }
        return nil
    })
}

