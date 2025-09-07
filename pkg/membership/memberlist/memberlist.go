package memberlist

import (
    "encoding/json"
    "context"
    "fmt"
    "log"
    "net"
    "sync"
    "time"

    base "github.com/amirimatin/go-cluster/pkg/membership"
    "github.com/hashicorp/memberlist"
)

// Options configures the memberlist-based membership implementation.
type Options struct {
    // NodeID is the unique node identifier.
    NodeID string

    // Bind is the bind address in host:port form (e.g. ":7946" or "0.0.0.0:7946").
    Bind string

    // Advertise is the advertised address (host:port) that peers will use to reach this node.
    // If empty, memberlist derives it from Bind.
    Advertise string

    // Meta is optional metadata associated with the node.
    Meta map[string]string

    // Logger is optional. If nil, log.Default() is used.
    Logger *log.Logger

    // Tuning parameters (optional). Zero means use defaults.
    ProbeInterval time.Duration
    ProbeTimeout  time.Duration
    SuspicionMult int
}

// impl implements base.Membership using HashiCorp memberlist.
type impl struct {
    mu     sync.RWMutex
    opts   Options
    ml     *memberlist.Memberlist
    evts   chan base.Event
    closed bool
}

// New constructs a memberlist-backed membership.
func New(opts Options) (base.Membership, error) {
    if opts.NodeID == "" {
        return nil, fmt.Errorf("memberlist: empty NodeID")
    }
    if opts.Bind == "" {
        return nil, fmt.Errorf("memberlist: empty Bind address")
    }
    if opts.Logger == nil {
        opts.Logger = log.Default()
    }
    return &impl{
        opts: opts,
        evts: make(chan base.Event, 64),
    }, nil
}

// Start creates and launches the underlying memberlist instance.
func (m *impl) Start(ctx context.Context) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    if m.ml != nil {
        return nil
    }

    cfg := memberlist.DefaultLANConfig()
    cfg.Name = m.opts.NodeID
    host, portStr, err := net.SplitHostPort(m.opts.Bind)
    if err != nil {
        return fmt.Errorf("memberlist: invalid bind address %q: %w", m.opts.Bind, err)
    }
    port, err := parsePort(portStr)
    if err != nil {
        return err
    }
    cfg.BindAddr = host
    cfg.BindPort = port

    if m.opts.Advertise != "" {
        ahost, aportStr, err := net.SplitHostPort(m.opts.Advertise)
        if err != nil {
            return fmt.Errorf("memberlist: invalid advertise address %q: %w", m.opts.Advertise, err)
        }
        aport, err := parsePort(aportStr)
        if err != nil {
            return err
        }
        cfg.AdvertiseAddr = ahost
        cfg.AdvertisePort = aport
    }

    if m.opts.ProbeInterval > 0 {
        cfg.ProbeInterval = m.opts.ProbeInterval
    }
    if m.opts.ProbeTimeout > 0 {
        cfg.ProbeTimeout = m.opts.ProbeTimeout
    }
    if m.opts.SuspicionMult > 0 {
        cfg.SuspicionMult = m.opts.SuspicionMult
    }

    // Wire delegates: events and node meta propagation.
    cfg.Events = &eventDelegate{emit: m.emit}
    // Encode static metadata once (e.g., management address) and expose via NodeDelegate.
    metaBytes, _ := json.Marshal(m.opts.Meta)
    cfg.Delegate = &nodeDelegate{meta: metaBytes}

    // Create memberlist
    ml, err := memberlist.Create(cfg)
    if err != nil {
        return err
    }
    m.ml = ml

    // Close events channel when context is done or on Stop().
    go func() {
        <-ctx.Done()
        _ = m.Stop()
    }()

    return nil
}

func (m *impl) Join(seeds []string) error {
    m.mu.RLock()
    ml := m.ml
    m.mu.RUnlock()
    if ml == nil {
        return fmt.Errorf("memberlist: not started")
    }
    if len(seeds) == 0 {
        return nil
    }
    _, err := ml.Join(seeds)
    return err
}

func (m *impl) Local() base.MemberInfo {
    m.mu.RLock()
    defer m.mu.RUnlock()
    if m.ml == nil {
        return base.MemberInfo{}
    }
    n := m.ml.LocalNode()
    // Attempt to decode meta from local node as well in case memberlist enriched it
    meta := map[string]string{}
    if len(n.Meta) > 0 {
        _ = json.Unmarshal(n.Meta, &meta)
    } else if m.opts.Meta != nil {
        meta = m.opts.Meta
    }
    return base.MemberInfo{ID: n.Name, Addr: net.JoinHostPort(n.Addr.String(), fmt.Sprintf("%d", n.Port)), Meta: meta}
}

func (m *impl) Members() []base.MemberInfo {
    m.mu.RLock()
    defer m.mu.RUnlock()
    if m.ml == nil {
        return nil
    }
    nodes := m.ml.Members()
    out := make([]base.MemberInfo, 0, len(nodes))
    for _, n := range nodes {
        meta := map[string]string{}
        if len(n.Meta) > 0 {
            _ = json.Unmarshal(n.Meta, &meta)
        }
        out = append(out, base.MemberInfo{ID: n.Name, Addr: net.JoinHostPort(n.Addr.String(), fmt.Sprintf("%d", n.Port)), Meta: meta})
    }
    return out
}

func (m *impl) Events() <-chan base.Event { return m.evts }

func (m *impl) Leave() error {
    m.mu.RLock()
    ml := m.ml
    m.mu.RUnlock()
    if ml == nil {
        return nil
    }
    // best-effort: leave and give some time to broadcast
    _ = ml.Leave(time.Second)
    return nil
}

func (m *impl) Stop() error {
    m.mu.Lock()
    defer m.mu.Unlock()
    if m.closed {
        return nil
    }
    m.closed = true
    if m.ml != nil {
        _ = m.ml.Shutdown()
        m.ml = nil
    }
    // Close events channel to signal completion
    select {
    case <-time.After(0):
        // non-blocking path
    default:
    }
    close(m.evts)
    return nil
}

// HealthScore exposes memberlist's awareness score if available.
// Implements membership.HealthReporter.
func (m *impl) HealthScore() int {
    m.mu.RLock()
    defer m.mu.RUnlock()
    if m.ml == nil {
        return -1
    }
    return m.ml.GetHealthScore()
}

// eventDelegate adapts memberlist events to base.Event.
type eventDelegate struct{
    emit func(e base.Event)
}

func (d *eventDelegate) NotifyJoin(n *memberlist.Node) {
    if d.emit == nil || n == nil { return }
    meta := map[string]string{}
    if len(n.Meta) > 0 { _ = json.Unmarshal(n.Meta, &meta) }
    d.emit(base.Event{Type: base.EventJoin, Member: base.MemberInfo{ID: n.Name, Addr: net.JoinHostPort(n.Addr.String(), fmt.Sprintf("%d", n.Port)), Meta: meta}, At: time.Now()})
}

func (d *eventDelegate) NotifyLeave(n *memberlist.Node) {
    if d.emit == nil || n == nil { return }
    // We map leave to EventLeave (memberlist conflates explicit leave and failure/timeouts).
    meta := map[string]string{}
    if len(n.Meta) > 0 { _ = json.Unmarshal(n.Meta, &meta) }
    d.emit(base.Event{Type: base.EventLeave, Member: base.MemberInfo{ID: n.Name, Addr: net.JoinHostPort(n.Addr.String(), fmt.Sprintf("%d", n.Port)), Meta: meta}, At: time.Now()})
}

func (d *eventDelegate) NotifyUpdate(n *memberlist.Node) {
    if d.emit == nil || n == nil { return }
    // Treat update as a join-like visibility for now.
    meta := map[string]string{}
    if len(n.Meta) > 0 { _ = json.Unmarshal(n.Meta, &meta) }
    d.emit(base.Event{Type: base.EventJoin, Member: base.MemberInfo{ID: n.Name, Addr: net.JoinHostPort(n.Addr.String(), fmt.Sprintf("%d", n.Port)), Meta: meta}, At: time.Now()})
}

func (m *impl) emit(e base.Event) {
    defer func(){ recover() }()
    select {
    case m.evts <- e:
    default:
        // drop if channel is full to avoid blocking
        if m.opts.Logger != nil {
            m.opts.Logger.Printf("memberlist: dropping event %v: channel full", e.Type)
        }
    }
}

func parsePort(s string) (int, error) {
    var p int
    _, err := fmt.Sscanf(s, "%d", &p)
    if err != nil || p < 0 || p > 65535 {
        return 0, fmt.Errorf("invalid port: %q", s)
    }
    return p, nil
}

// nodeDelegate implements memberlist.Delegate to propagate node metadata (e.g., mgmt address).
type nodeDelegate struct{ meta []byte }

// NodeMeta is used to retrieve meta-data about the current node when broadcasting
// an alive message. The returned byte slice will be truncated to the given limit,
// as it will be broadcast in gossip.
func (d *nodeDelegate) NodeMeta(limit int) []byte {
    if len(d.meta) <= limit { return d.meta }
    if limit <= 0 { return nil }
    return d.meta[:limit]
}

// Unused hooks for our purposes; required to satisfy the interface.
func (d *nodeDelegate) NotifyMsg([]byte)                       {}
func (d *nodeDelegate) GetBroadcasts(int, int) [][]byte        { return nil }
func (d *nodeDelegate) LocalState(join bool) []byte            { return nil }
func (d *nodeDelegate) MergeRemoteState(buf []byte, join bool) {}
