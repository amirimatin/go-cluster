package grpc

import (
    "context"
    "sync"
    "time"

    "google.golang.org/grpc"

    obsmetrics "github.com/amirimatin/go-cluster/pkg/observability/metrics"
)

// ConnManager caches gRPC client connections per address with idle eviction.
type ConnManager struct {
    mu      sync.Mutex
    conns   map[string]*managedConn
    ttl     time.Duration
    dialer  func(ctx context.Context, target string) (*grpc.ClientConn, error)
    closing chan struct{}
}

type managedConn struct {
    cc       *grpc.ClientConn
    lastUsed time.Time
    ref      int
}

// NewConnManager creates a manager with the given idle TTL and dialer.
func NewConnManager(ttl time.Duration, dialer func(ctx context.Context, target string) (*grpc.ClientConn, error)) *ConnManager {
    if ttl <= 0 { ttl = 30 * time.Second }
    m := &ConnManager{ttl: ttl, dialer: dialer, conns: make(map[string]*managedConn), closing: make(chan struct{})}
    go m.janitor()
    return m
}

// Get returns a connection for target and a release func to be called when done.
func (m *ConnManager) Get(ctx context.Context, target string) (*grpc.ClientConn, func(), error) {
    m.mu.Lock()
    if mc, ok := m.conns[target]; ok && mc.cc != nil {
        mc.ref++
        mc.lastUsed = time.Now()
        cc := mc.cc
        m.mu.Unlock()
        return cc, func(){ m.release(target) }, nil
    }
    m.mu.Unlock()

    // Dial outside lock
    cc, err := m.dialer(ctx, target)
    if err != nil { return nil, func(){}, err }

    m.mu.Lock()
    if existing, ok := m.conns[target]; ok && existing.cc != nil {
        // Race: another goroutine created it. Use existing and close ours.
        _ = cc.Close()
        existing.ref++
        existing.lastUsed = time.Now()
        out := existing.cc
        m.mu.Unlock()
        obsmetrics.GRPCConnReuse.Inc()
        return out, func(){ m.release(target) }, nil
    }
    m.conns[target] = &managedConn{cc: cc, lastUsed: time.Now(), ref: 1}
    obsmetrics.GRPCConnDials.Inc()
    obsmetrics.GRPCConnActive.Inc()
    m.mu.Unlock()
    return cc, func(){ m.release(target) }, nil
}

func (m *ConnManager) release(target string) {
    m.mu.Lock()
    if mc, ok := m.conns[target]; ok {
        if mc.ref > 0 { mc.ref-- }
        mc.lastUsed = time.Now()
    }
    m.mu.Unlock()
}

// Close closes all cached connections and stops the janitor.
func (m *ConnManager) Close() {
    close(m.closing)
    m.mu.Lock()
    for k, mc := range m.conns {
        if mc.cc != nil { _ = mc.cc.Close() }
        delete(m.conns, k)
    }
    m.mu.Unlock()
}

func (m *ConnManager) janitor() {
    ticker := time.NewTicker(m.ttl / 2)
    defer ticker.Stop()
    for {
        select {
        case <-m.closing:
            return
        case <-ticker.C:
            cutoff := time.Now().Add(-m.ttl)
            m.mu.Lock()
            for addr, mc := range m.conns {
                if mc.ref == 0 && mc.lastUsed.Before(cutoff) {
                    if mc.cc != nil { _ = mc.cc.Close() }
                    obsmetrics.GRPCConnEvictions.Inc()
                    obsmetrics.GRPCConnActive.Dec()
                    delete(m.conns, addr)
                }
            }
            m.mu.Unlock()
        }
    }
}
