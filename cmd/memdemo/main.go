package main

import (
    "context"
    "flag"
    "fmt"
    "log"
    "os"
    "os/signal"
    "strings"
    "syscall"
    "time"

    base "github.com/amirimatin/go-cluster/pkg/membership"
    ml "github.com/amirimatin/go-cluster/pkg/membership/memberlist"
)

func main() {
    var (
        id        = flag.String("id", "node-1", "node id")
        bind      = flag.String("bind", ":7946", "bind host:port")
        advertise = flag.String("advertise", "", "advertise host:port (optional)")
        joinCSV   = flag.String("join", "", "comma-separated seeds (host:port)")
    )
    flag.Parse()

    ctx, cancel := signalContext()
    defer cancel()

    m, err := ml.New(ml.Options{NodeID: *id, Bind: *bind, Advertise: *advertise, Logger: log.Default()})
    if err != nil { log.Fatal(err) }
    if err := m.Start(ctx); err != nil { log.Fatal(err) }

    if *joinCSV != "" {
        seeds := splitCSV(*joinCSV)
        if err := m.Join(seeds); err != nil { log.Printf("join error: %v", err) }
    }

    fmt.Println("memdemo started. Press Ctrl+C to exit.")
    go func(evch <-chan base.Event) {
        for e := range evch {
            fmt.Printf("event: %-6s id=%s addr=%s at=%s\n", e.Type, e.Member.ID, e.Member.Addr, e.At.Format(time.RFC3339))
        }
    }(m.Events())

    <-ctx.Done()
    _ = m.Leave()
    _ = m.Stop()
}

func splitCSV(s string) []string {
    if s == "" { return nil }
    parts := strings.Split(s, ",")
    out := make([]string, 0, len(parts))
    for _, p := range parts { p = strings.TrimSpace(p); if p != "" { out = append(out, p) } }
    return out
}

func signalContext() (context.Context, context.CancelFunc) {
    ctx, cancel := context.WithCancel(context.Background())
    go func() {
        ch := make(chan os.Signal, 1)
        signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
        <-ch
        cancel()
    }()
    return ctx, cancel
}

