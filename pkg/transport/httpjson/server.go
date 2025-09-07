package httpjson

import (
    "crypto/tls"
    "context"
    "encoding/json"
    "fmt"
    "log"
    "net"
    "net/http"
    "time"

    "github.com/prometheus/client_golang/prometheus/promhttp"

    "github.com/amirimatin/go-cluster/pkg/transport"
    "github.com/amirimatin/go-cluster/pkg/observability/tracing"
)

// Server is a minimal HTTP server exposing management endpoints for status,
// join/leave and metrics/healthz. It is intended for intra-cluster calls and
// development tooling.
type Server struct {
    bind   string
    srv    *http.Server
    logger *log.Logger
    tlsCfg *tls.Config
}

// NewServer binds to the given TCP address (e.g., ":17946").
func NewServer(bind string, logger *log.Logger) *Server {
    if logger == nil { logger = log.Default() }
    return &Server{bind: bind, logger: logger}
}

// UseTLS enables TLS for the HTTP server using the provided config.
func (s *Server) UseTLS(cfg *tls.Config) *Server { s.tlsCfg = cfg; return s }

// Start launches the HTTP server and registers handlers backed by the provided
// functions. The server is shut down when the context is canceled.
func (s *Server) Start(ctx context.Context, status transport.StatusFunc, join transport.JoinFunc, leave transport.LeaveFunc, appWrite transport.AppWriteFunc, appSync transport.AppSyncFunc) error {
    mux := http.NewServeMux()
    mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodGet { http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return }
        ctx, end := tracing.StartSpan(r.Context(), "http.status")
        defer end()
        data, err := status(ctx)
        if err != nil { http.Error(w, fmt.Sprintf("status error: %v", err), http.StatusInternalServerError); return }
        w.Header().Set("Content-Type", "application/json")
        _, _ = w.Write(data)
    })
    mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodGet { http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return }
        w.WriteHeader(http.StatusOK)
        _, _ = w.Write([]byte("ok"))
    })
    // Prometheus metrics
    mux.Handle("/metrics", promhttp.Handler())
    mux.HandleFunc("/join", func(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodPost { http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return }
        if join == nil { http.Error(w, "join not supported", http.StatusNotImplemented); return }
        var req transport.JoinRequest
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            http.Error(w, fmt.Sprintf("bad request: %v", err), http.StatusBadRequest)
            return
        }
        ctx, end := tracing.StartSpan(r.Context(), "http.join")
        defer end()
        resp, err := join(ctx, req)
        if err != nil {
            w.Header().Set("Content-Type", "application/json")
            w.WriteHeader(http.StatusInternalServerError)
            _ = json.NewEncoder(w).Encode(resp)
            return
        }
        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode(resp)
    })
    mux.HandleFunc("/leave", func(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodPost { http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return }
        if leave == nil { http.Error(w, "leave not supported", http.StatusNotImplemented); return }
        var req transport.LeaveRequest
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            http.Error(w, fmt.Sprintf("bad request: %v", err), http.StatusBadRequest)
            return
        }
        ctx, end := tracing.StartSpan(r.Context(), "http.leave")
        defer end()
        resp, err := leave(ctx, req)
        if err != nil {
            w.Header().Set("Content-Type", "application/json")
            w.WriteHeader(http.StatusInternalServerError)
            _ = json.NewEncoder(w).Encode(resp)
            return
        }
        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode(resp)
    })
    mux.HandleFunc("/appwrite", func(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodPost { http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return }
        if appWrite == nil { http.Error(w, "appwrite not supported", http.StatusNotImplemented); return }
        var req transport.AppWriteRequest
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            http.Error(w, fmt.Sprintf("bad request: %v", err), http.StatusBadRequest)
            return
        }
        resp, err := appWrite(r.Context(), req)
        w.Header().Set("Content-Type", "application/json")
        if err != nil {
            // Include any data plus error
            if resp.Error == "" { resp.Error = err.Error() }
            w.WriteHeader(http.StatusInternalServerError)
            _ = json.NewEncoder(w).Encode(resp)
            return
        }
        _ = json.NewEncoder(w).Encode(resp)
    })
    mux.HandleFunc("/appsync", func(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodPost { http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return }
        if appSync == nil { http.Error(w, "appsync not supported", http.StatusNotImplemented); return }
        var req transport.AppSyncRequest
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            http.Error(w, fmt.Sprintf("bad request: %v", err), http.StatusBadRequest)
            return
        }
        resp, err := appSync(r.Context(), req)
        w.Header().Set("Content-Type", "application/json")
        if err != nil {
            if resp.Error == "" { resp.Error = err.Error() }
            w.WriteHeader(http.StatusInternalServerError)
            _ = json.NewEncoder(w).Encode(resp)
            return
        }
        _ = json.NewEncoder(w).Encode(resp)
    })

    s.srv = &http.Server{Addr: s.bind, Handler: mux}

    ln, err := net.Listen("tcp", s.bind)
    if err != nil { return err }
    if s.tlsCfg != nil {
        ln = tls.NewListener(ln, s.tlsCfg)
    }

    go func() {
        <-ctx.Done()
        _ = s.Stop(context.Background())
    }()
    go func() {
        if err := s.srv.Serve(ln); err != nil && err != http.ErrServerClosed {
            s.logger.Printf("httpjson: server error: %v", err)
        }
    }()
    return nil
}

// Addr returns the configured bind address.
func (s *Server) Addr() string { return s.bind }

// Stop attempts a graceful shutdown with a short timeout.
func (s *Server) Stop(ctx context.Context) error {
    if s.srv == nil { return nil }
    c, cancel := context.WithTimeout(ctx, 2*time.Second)
    defer cancel()
    err := s.srv.Shutdown(c)
    s.srv = nil
    return err
}

var _ transport.RPCServer = (*Server)(nil)
