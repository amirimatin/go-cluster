package raftcons

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    "os"
    "path/filepath"
    "strconv"
    "time"

    "github.com/hashicorp/raft"
    raftboltdb "github.com/hashicorp/raft-boltdb"

    c "github.com/amirimatin/go-cluster/pkg/consensus"
    sm "github.com/amirimatin/go-cluster/pkg/state/membership"
    baseState "github.com/amirimatin/go-cluster/pkg/state"
)

// Node implements consensus.Consensus using HashiCorp Raft.
// In this initial phase it uses in-memory stores and transports suitable for
// single-node operation. Multi-node wiring will be added in later phases.
type Node struct {
    opts Options
    log  *log.Logger
    r    *raft.Raft
    lch  chan c.LeaderInfo
    // transport details (loopback only in current phase)
    addr raft.ServerAddress
    trans raft.Transport
    lb   raft.LoopbackTransport
    ms   baseState.MembershipState
}

func New(opts Options) (*Node, error) {
    if opts.NodeID == "" {
        return nil, fmt.Errorf("raftcons: empty NodeID")
    }
    if opts.Logger == nil {
        opts.Logger = log.Default()
    }
    return &Node{opts: opts, log: opts.Logger, lch: make(chan c.LeaderInfo, 16)}, nil
}

func (n *Node) Start(ctx context.Context) error {
    if n.r != nil {
        return nil
    }

    // Raft configuration
    cfg := raft.DefaultConfig()
    cfg.LocalID = raft.ServerID(n.opts.NodeID)
    if n.opts.HeartbeatTimeout > 0 {
        cfg.HeartbeatTimeout = n.opts.HeartbeatTimeout
        // Keep lease <= heartbeat to satisfy invariants
        if cfg.LeaderLeaseTimeout > cfg.HeartbeatTimeout {
            cfg.LeaderLeaseTimeout = cfg.HeartbeatTimeout / 2
            if cfg.LeaderLeaseTimeout == 0 { cfg.LeaderLeaseTimeout = cfg.HeartbeatTimeout }
        }
    }
    if n.opts.ElectionTimeout > 0 { cfg.ElectionTimeout = n.opts.ElectionTimeout }
    if n.opts.CommitTimeout > 0 { cfg.CommitTimeout = n.opts.CommitTimeout }

    // Stores and transport
    var (
        logs raft.LogStore
        stable raft.StableStore
        snaps raft.SnapshotStore
        addr raft.ServerAddress
        trans raft.Transport
        err error
    )

    // Storage selection: on-disk when DataDir provided, else in-memory.
    if n.opts.DataDir != "" {
        if n.opts.SnapshotsRetained == 0 { n.opts.SnapshotsRetained = 2 }
        if err := os.MkdirAll(n.opts.DataDir, 0o755); err != nil { return err }
        // Bolt store for both log and stable
        bpath := filepath.Join(n.opts.DataDir, "raft.db")
        bstore, err := raftboltdb.NewBoltStore(bpath)
        if err != nil { return err }
        logs = bstore
        stable = bstore
        snaps, err = raft.NewFileSnapshotStore(n.opts.DataDir, n.opts.SnapshotsRetained, os.Stderr)
        if err != nil { return err }
    } else {
        logs = raft.NewInmemStore()
        stable = raft.NewInmemStore()
        snaps = raft.NewInmemSnapshotStore()
    }

    // Transport selection
    if n.opts.BindAddr != "" {
        // TCP transport with dynamic advertise if port is :0
        nt, err := raft.NewTCPTransport(n.opts.BindAddr, nil, 3, 1*time.Second, os.Stderr)
        if err != nil { return err }
        trans = nt
        addr = nt.LocalAddr()
    } else {
        addr, trans = raft.NewInmemTransport(raft.ServerAddress(n.opts.NodeID))
    }

    // FSM backed by membership state
    // Membership FSM instance
    n.ms = sm.New()
    fsm := newMembershipFSM(n.ms)

    r, err := raft.NewRaft(cfg, fsm, logs, stable, snaps, trans)
    if err != nil {
        return err
    }
    n.r = r
    n.addr = addr
    n.trans = trans
    if lb, ok := n.trans.(raft.LoopbackTransport); ok { n.lb = lb }

    // Observe leadership/state changes and forward to LeaderCh.
    obsCh := make(chan raft.Observation, 32)
    observer := raft.NewObserver(obsCh, false, func(o *raft.Observation) bool {
        switch o.Data.(type) {
        case raft.LeaderObservation:
            return true
        default:
            return false
        }
    })
    n.r.RegisterObserver(observer)
    go func() {
        for range obsCh {
            // Emit leader info on relevant observations.
            id, addr, ok := n.Leader()
            if ok {
                n.emitLeader(c.LeaderInfo{ID: id, Addr: addr, Term: n.Term()})
            }
        }
    }()

    // Also emit an initial leader snapshot if known shortly after start.
    go func() {
        // Small delay to allow Raft to settle, then emit if leader.
        time.Sleep(50 * time.Millisecond)
        id, addr, ok := n.Leader()
        if ok {
            n.emitLeader(c.LeaderInfo{ID: id, Addr: addr, Term: n.Term()})
        }
    }()

    if n.opts.Bootstrap {
        cfgs := raft.Configuration{Servers: []raft.Server{{
            ID:      cfg.LocalID,
            Address: addr,
        }}}
        if err := n.r.BootstrapCluster(cfgs).Error(); err != nil {
            return err
        }
    }

    go func() {
        <-ctx.Done()
        _ = n.Stop()
    }()
    return nil
}

func (n *Node) Apply(cmd c.Command, timeout time.Duration) error {
    if n.r == nil {
        return fmt.Errorf("raftcons: not started")
    }
    if n.r.State() != raft.Leader {
        return fmt.Errorf("raftcons: not leader")
    }
    data, err := json.Marshal(cmd)
    if err != nil { return err }
    t := timeout
    if t <= 0 && n.opts.ApplyTimeout > 0 { t = n.opts.ApplyTimeout }
    af := n.r.Apply(data, t)
    if err := af.Error(); err != nil { return err }
    if v := af.Response(); v != nil {
        if e, ok := v.(error); ok && e != nil { return e }
    }
    return nil
}

func (n *Node) IsLeader() bool {
    if n.r == nil { return false }
    return n.r.State() == raft.Leader
}

func (n *Node) Leader() (id string, addr string, ok bool) {
    if n.r == nil { return "", "", false }
    a, sid := n.r.LeaderWithID()
    if sid == "" { return "", "", false }
    return string(sid), string(a), true
}

func (n *Node) Term() uint64 {
    if n.r == nil { return 0 }
    // Try to parse from stats; falls back to 0.
    if v := n.r.Stats()["current_term"]; v != "" {
        if u, err := strconv.ParseUint(v, 10, 64); err == nil { return u }
    }
    return 0
}

func (n *Node) Stop() error {
    if n.r == nil { return nil }
    f := n.r.Shutdown()
    if err := f.Error(); err != nil { return err }
    n.r = nil
    return nil
}

// Ensure interface compliance
var _ c.Consensus = (*Node)(nil)
// Also implements optional LeaderNotifier.
func (n *Node) LeaderCh() <-chan c.LeaderInfo { return n.lch }

func (n *Node) emitLeader(li c.LeaderInfo) {
    select {
    case n.lch <- li:
    default:
        // drop to avoid blocking; last-writer-wins semantics are ok for leadership
    }
}

// StateSnapshot returns the current membership snapshot (for testing/inspection).
func (n *Node) StateSnapshot() ([]byte, error) {
    if n.ms == nil { return nil, fmt.Errorf("no state") }
    return n.ms.Snapshot()
}

// --- Dynamic Reconfiguration (optional) ---

// AddVoter adds a voting server to the Raft cluster if not already present.
func (n *Node) AddVoter(id, addr string, timeout time.Duration) error {
    if n.r == nil {
        return fmt.Errorf("raftcons: not started")
    }
    // Fast-path: if exists with same address, accept.
    cfg := n.r.GetConfiguration()
    if err := cfg.Error(); err == nil {
        for _, srv := range cfg.Configuration().Servers {
            if string(srv.ID) == id {
                if string(srv.Address) == addr {
                    return nil
                }
                // Remove stale entry with different address before adding
                rf := n.r.RemoveServer(srv.ID, 0, timeout)
                if err := rf.Error(); err != nil { return err }
                break
            }
        }
    }
    f := n.r.AddVoter(raft.ServerID(id), raft.ServerAddress(addr), 0, timeout)
    return f.Error()
}

// RemoveServer removes a server from the Raft cluster if present.
func (n *Node) RemoveServer(id string, timeout time.Duration) error {
    if n.r == nil {
        return fmt.Errorf("raftcons: not started")
    }
    f := n.r.RemoveServer(raft.ServerID(id), 0, timeout)
    return f.Error()
}
