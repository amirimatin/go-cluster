package main

import (
    "context"
    "fmt"
    "log"
    "net/http"
    "os/signal"
    "syscall"
    "time"

    "github.com/amirimatin/go-cluster/pkg/bootstrap"
)

// This example shows how a microservice can embed the cluster node using the
// reusable bootstrap package, without depending on the CLI.
// demoHandlers is a trivial implementation of AppHandlers.
type demoHandlers struct{}

func (demoHandlers) HandleWrite(ctx context.Context, op string, req []byte) ([]byte, error) {
    // Echo with timestamp
    return []byte(fmt.Sprintf("op=%s t=%d payload=%s", op, time.Now().Unix(), string(req))), nil
}
func (demoHandlers) HandleRead(ctx context.Context, op string, req []byte) ([]byte, error)  { return []byte("read-not-impl"), nil }
func (demoHandlers) HandleSync(ctx context.Context, topic string, data []byte) error        {
    log.Printf("SYNC topic=%s data=%s\n", topic, string(data))
    return nil
}

func main() {
    ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
    defer cancel()

    cl, err := bootstrap.Run(ctx, bootstrap.Config{
        NodeID:   "svc-1",
        RaftAddr: ":9521",
        MemBind:  ":7946",
        MgmtAddr: ":17946", // management should be separate from membership
        // Choose a discovery backend suitable for your environment:
        DiscoveryKind: "static",
        SeedsCSV:      "127.0.0.1:7946",
        // Persist Raft on disk in production
        DataDir:  "",
        Bootstrap: true, // single-node demo
        AppHandlers: demoHandlers{},
    })
    if err != nil { log.Fatalf("cluster bootstrap: %v", err) }
    defer cl.Close()

    // Subscribe to cluster events
    evs := cl.Subscribe(ctx)
    go func() {
        for e := range evs {
            log.Printf("EVENT type=%s term=%d leader=%v member=%v\n", e.Type, e.Term, e.Leader, e.Member)
        }
    }()

    // Demo endpoints to trigger AppWrite/Publish
    http.HandleFunc("/write", func(w http.ResponseWriter, r *http.Request) {
        out, err := cl.AppWrite(r.Context(), "demo", []byte("hello"))
        if err != nil { w.WriteHeader(500); _, _ = w.Write([]byte(err.Error())); return }
        _, _ = w.Write(out)
    })
    http.HandleFunc("/publish", func(w http.ResponseWriter, r *http.Request) {
        if err := cl.Publish(r.Context(), "demo.topic", []byte("sync-data")); err != nil {
            w.WriteHeader(500); _, _ = w.Write([]byte(err.Error())); return
        }
        _, _ = w.Write([]byte("ok"))
    })
    http.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
        s, _ := cl.Status(r.Context())
        _, _ = w.Write([]byte(fmt.Sprintf("leader=%s term=%d members=%d\n", s.LeaderID, s.Term, len(s.Members))))
    })

    // Your microservice application logic starts here
    http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK); _, _ = w.Write([]byte("ok")) })
    log.Println("service listening at :8080 (example) — endpoints: /status /write /publish /healthz")
    _ = http.ListenAndServe(":8080", nil)
}
