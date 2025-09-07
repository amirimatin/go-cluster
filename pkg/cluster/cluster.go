package cluster

import (
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "os"
    "path/filepath"
    "strconv"
    "strings"
    "sync"
    "time"

    "github.com/amirimatin/go-cluster/pkg/consensus"
    "github.com/amirimatin/go-cluster/pkg/membership"
    obsmetrics "github.com/amirimatin/go-cluster/pkg/observability/metrics"
    "github.com/amirimatin/go-cluster/pkg/observability/tracing"
    "github.com/amirimatin/go-cluster/pkg/internal/logutil"
    "github.com/amirimatin/go-cluster/pkg/transport"
)

// Facade exposes the high-level API for consumers (see go-cluster.md §11).
type Facade interface {
    Start(ctx context.Context) error
    Join(ctx context.Context, seedLeader string) error
    Status(ctx context.Context) (*ClusterStatus, error)
    Stop(ctx context.Context) error
    LeaderCh() <-chan consensus.LeaderInfo
}

// Cluster is the concrete implementation of the Facade. It wires together
// membership, consensus (RAFT), management RPC and application callbacks to
// provide a small, embeddable clustering runtime for microservices.
type Cluster struct {
    opts Options
    mu   sync.RWMutex
    run  struct {
        started bool
        closed  bool
    }
    cons consensus.Consensus
    mem  membership.Membership
    rpcS transport.RPCServer
    rpcC transport.RPCClient
    eb   eventBus
    rep struct{
        mu   sync.Mutex
        seq  map[string]uint64
        buf  map[string]map[uint64][]byte
    }
    el struct{
        mu        sync.Mutex
        hadLeader bool
        inProgress bool
    }
}

// New constructs a new Cluster instance from validated options. It performs no
// network activity; call Start to launch the node.
func New(ctx context.Context, opts Options) (*Cluster, error) {
    if err := opts.Validate(); err != nil {
        return nil, err
    }
    c := &Cluster{opts: opts, cons: opts.Consensus, mem: opts.Membership, rpcS: opts.RPCServer, rpcC: opts.RPCClient}
    c.rep.seq = make(map[string]uint64)
    c.rep.buf = make(map[string]map[uint64][]byte)
    // In future phases, we may start background tasks here.
    return c, nil
}

// Close is a convenience alias for Stop with a background context.
func (c *Cluster) Close() error {
    return c.Stop(context.Background())
}

// Start launches membership, consensus (RAFT) and the management endpoint, then
// begins internal loops for reconciling membership and replication.
func (c *Cluster) Start(ctx context.Context) error {
    c.mu.Lock()
    defer c.mu.Unlock()
    if c.run.started {
        return nil
    }
    c.run.started = true
    // Register metrics once
    obsmetrics.Register()
    // Prepare disk buffer directory if configured and restore outstanding
    if dir := c.opts.ReplBufferDir; dir != "" {
        _ = os.MkdirAll(dir, 0o755)
        c.restoreBufferFromDisk(dir)
    }
    // Start membership and join seeds
    if c.mem != nil {
        if err := c.mem.Start(ctx); err != nil { return err }
        if seeds := c.opts.Discovery.Seeds(); len(seeds) > 0 {
            logutil.Infof(c.opts.Logger, "joining membership seeds: %v", seeds)
            _ = c.mem.Join(seeds)
        }
    }
    // Start consensus engine in background lifecycle.
    if c.cons != nil {
        if err := c.cons.Start(ctx); err != nil { return err }
        go c.reconcileMembersLoop(ctx)
        go c.membershipEventsLoop(ctx)
        go c.replicationSubscribeLoop(ctx)
        go c.replicationRetryLoop(ctx)
        go c.electionWatchLoop(ctx)
        // Observe leader changes if supported
        if ln, ok := c.cons.(consensus.LeaderNotifier); ok {
            go func() {
                for li := range ln.LeaderCh() {
                    obsmetrics.LeaderChanges.Inc()
                    logutil.Infof(c.opts.Logger, "leader change observed: id=%s term=%d", li.ID, li.Term)
                    // Publish app-facing event
                    liCopy := li
                    c.eb.publish(Event{Type: EventLeaderChanged, At: time.Now(), Leader: &liCopy, Term: li.Term})
                    // Election end callback
                    if c.opts.OnLeaderChange != nil { c.opts.OnLeaderChange(liCopy) }
                    if c.opts.OnElectionEnd != nil { c.opts.OnElectionEnd(liCopy) }
                    c.el.mu.Lock(); c.el.inProgress = false; c.el.hadLeader = true; c.el.mu.Unlock()
                }
            }()
        }
    }

    // Start management RPC server (if configured)
    if c.rpcS != nil {
        statusFn := func(ctx context.Context) ([]byte, error) { return c.statusLocalJSON(ctx) }
        joinFn := func(ctx context.Context, req transport.JoinRequest) (transport.JoinResponse, error) { return c.handleJoin(ctx, req) }
        leaveFn := func(ctx context.Context, req transport.LeaveRequest) (transport.LeaveResponse, error) { return c.handleLeave(ctx, req) }
        appWriteFn := func(ctx context.Context, req transport.AppWriteRequest) (transport.AppWriteResponse, error) { return c.handleAppWrite(ctx, req) }
        appSyncFn := func(ctx context.Context, req transport.AppSyncRequest) (transport.AppSyncResponse, error) { return c.handleAppSync(ctx, req) }
        if err := c.rpcS.Start(ctx, statusFn, joinFn, leaveFn, appWriteFn, appSyncFn); err != nil { return err }
        logutil.Infof(c.opts.Logger, "management endpoint listening at %s (status/metrics/healthz)", c.rpcS.Addr())
    }
    return nil
}

// Join requests to add this node as a voter to the RAFT cluster via the current
// leader's management endpoint. When seedLeader is empty, the method attempts
// to resolve the leader using consensus and membership metadata.
func (c *Cluster) Join(ctx context.Context, seedLeader string) error {
    if c.rpcC == nil {
        return errors.New("cluster: no RPC client configured")
    }
    // Discover leader management address
    leaderMgmt := seedLeader
    if leaderMgmt == "" {
        if c.cons == nil { return errors.New("cluster: no consensus to discover leader") }
        if id, _, ok := c.cons.Leader(); ok {
            leaderMgmt = c.lookupMemberAddr(id)
        }
    } else {
        // Resolve leader via seed's status endpoint to ensure we target the actual leader
        if data, err := c.rpcC.GetStatus(ctx, leaderMgmt); err == nil {
            var st ClusterStatus
            if json.Unmarshal(data, &st) == nil && st.LeaderAddr != "" {
                leaderMgmt = st.LeaderAddr
            }
        }
    }
    if leaderMgmt == "" {
        return errors.New("cluster: cannot resolve leader management address")
    }
    req := transport.JoinRequest{ID: string(c.opts.NodeID), RaftAddr: c.opts.Transport.Addr()}
    resp, err := c.rpcC.PostJoin(ctx, leaderMgmt, req)
    if err != nil {
        return err
    }
    if !resp.Accepted {
        if resp.Error == "not leader" {
            return ErrNotLeader
        }
        if resp.Error != "" {
            return errors.New(resp.Error)
        }
        return errors.New("cluster: join rejected")
    }
    return nil
}

// Status returns a synthesized snapshot including RAFT term/leader and the
// membership view. When called on a follower, it proxies to the leader to
// obtain a canonical view (including LeaderAddr), when possible.
func (c *Cluster) Status(ctx context.Context) (*ClusterStatus, error) {
    s := &ClusterStatus{}
    if c.cons != nil {
        s.Term = c.cons.Term()
        if id, _, ok := c.cons.Leader(); ok {
            s.LeaderID = id
            s.Healthy = true
            // If leader locally, expose management address as LeaderAddr
            if c.cons.IsLeader() && c.rpcS != nil {
                s.LeaderAddr = c.rpcS.Addr()
            } else if c.rpcC != nil && c.mem != nil {
                // Not leader: proxy to leader to get canonical view (including mgmt address)
                if la := c.lookupMemberAddr(id); la != "" {
                    if data, err := c.rpcC.GetStatus(ctx, la); err == nil {
                        var rs ClusterStatus
                        if json.Unmarshal(data, &rs) == nil {
                            return &rs, nil
                        }
                    }
                }
            }
        }
    }
    if c.mem != nil {
        s.Members = c.mem.Members()
        // Update metrics with current view
        obsmetrics.ClusterMembers.Set(float64(len(s.Members)))
    }
    // is leader gauge
    if c.cons != nil && c.cons.IsLeader() {
        obsmetrics.IsLeader.Set(1)
    } else {
        obsmetrics.IsLeader.Set(0)
    }
    return s, nil
}

// Stop gracefully shuts down consensus, membership and the management server.
func (c *Cluster) Stop(ctx context.Context) error {
    c.mu.Lock()
    defer c.mu.Unlock()
    if c.run.closed {
        return nil
    }
    c.run.closed = true
    if c.cons != nil {
        _ = c.cons.Stop()
    }
    if c.mem != nil {
        _ = c.mem.Leave()
        _ = c.mem.Stop()
    }
    if c.rpcS != nil {
        _ = c.rpcS.Stop(ctx)
    }
    return nil
}

func (c *Cluster) membershipEventsLoop(ctx context.Context) {
    if c.mem == nil { return }
    evch := c.mem.Events()
    for {
        select {
        case <-ctx.Done():
            return
        case e, ok := <-evch:
            if !ok { return }
            if c.cons == nil || !c.cons.IsLeader() { continue }
            switch e.Type {
            case membership.EventJoin:
                c.applyAddNode(e.Member)
                // Update members gauge on join events
                if c.mem != nil { obsmetrics.ClusterMembers.Set(float64(len(c.mem.Members()))) }
                // Publish app-facing event
                m := e.Member
                c.eb.publish(Event{Type: EventMemberJoin, At: e.At, Member: &m})
            case membership.EventLeave, membership.EventFailed:
                // On departure, remove from Raft then apply state removal
                c.removeServer(e.Member.ID)
                c.applyRemoveNode(e.Member.ID)
                if c.mem != nil { obsmetrics.ClusterMembers.Set(float64(len(c.mem.Members()))) }
                // Publish app-facing event
                et := EventMemberLeave
                if e.Type == membership.EventFailed { et = EventMemberFailed }
                m := e.Member
                c.eb.publish(Event{Type: et, At: e.At, Member: &m})
            }
        }
    }
}

func (c *Cluster) reconcileMembersLoop(ctx context.Context) {
    // allow membership to settle minimally
    time.Sleep(200 * time.Millisecond)
    if c.cons == nil || !c.cons.IsLeader() || c.mem == nil { return }
    for _, m := range c.mem.Members() {
        c.applyAddNode(m)
    }
}

func (c *Cluster) applyAddNode(mi membership.MemberInfo) {
    if c.cons == nil || !c.cons.IsLeader() { return }
    b, _ := json.Marshal(mi)
    _ = c.cons.Apply(consensus.Command{Op: "AddNode", Payload: b}, 2*time.Second)
}

func (c *Cluster) applyRemoveNode(id string) {
    if c.cons == nil || !c.cons.IsLeader() { return }
    b, _ := json.Marshal(struct{ ID string `json:"id"` }{ID: id})
    _ = c.cons.Apply(consensus.Command{Op: "RemoveNode", Payload: b}, 2*time.Second)
}

func (c *Cluster) statusLocalJSON(ctx context.Context) ([]byte, error) {
    st, err := c.Status(ctx)
    if err != nil { return nil, err }
    return json.Marshal(st)
}

// lookupMemberAddr returns the target management address for a given member ID.
// It prefers membership Meta["mgmt"] when available; otherwise falls back to
// the membership gossip address (which may not serve management APIs).
func (c *Cluster) lookupMemberAddr(id string) string {
    if c.mem == nil { return "" }
    ms := c.mem.Members()
    for _, m := range ms {
        if m.ID == id {
            if m.Meta != nil {
                if mgmt := m.Meta["mgmt"]; mgmt != "" { return mgmt }
            }
            return m.Addr
        }
    }
    return ""
}

func (c *Cluster) handleJoin(ctx context.Context, req transport.JoinRequest) (transport.JoinResponse, error) {
    ctx, end := tracing.StartSpan(ctx, "cluster.handleJoin")
    defer end()
    // Only leader accepts join requests
    if c.cons == nil || !c.cons.IsLeader() {
        // Try to hint the leader management address to client
        var leaderMgmt string
        if c.cons != nil {
            if id, _, ok := c.cons.Leader(); ok {
                leaderMgmt = c.lookupMemberAddr(id)
            }
        }
        obsmetrics.JoinRequests.WithLabelValues("rejected").Inc()
        logutil.Warnf(c.opts.Logger, "join rejected (not leader): id=%s", req.ID)
        return transport.JoinResponse{Accepted: false, Leader: leaderMgmt, Error: "not leader"}, nil
    }
    // Reconfigure Raft cluster to include the new voter
    if rc, ok := c.cons.(consensus.Reconfigurer); ok {
        if err := rc.AddVoter(req.ID, req.RaftAddr, 3*time.Second); err != nil {
            logutil.Errorf(c.opts.Logger, "add voter failed: id=%s addr=%s err=%v", req.ID, req.RaftAddr, err)
            return transport.JoinResponse{Accepted: false, Error: err.Error()}, nil
        }
    }
    // Membership state change will be applied via membership event when gossip converges
    obsmetrics.JoinRequests.WithLabelValues("accepted").Inc()
    logutil.Infof(c.opts.Logger, "join accepted: id=%s addr=%s", req.ID, req.RaftAddr)
    return transport.JoinResponse{Accepted: true}, nil
}

func (c *Cluster) removeServer(id string) {
    if c.cons == nil || !c.cons.IsLeader() { return }
    if rc, ok := c.cons.(consensus.Reconfigurer); ok {
        if err := rc.RemoveServer(id, 3*time.Second); err != nil {
            logutil.Warnf(c.opts.Logger, "remove voter failed: id=%s err=%v", id, err)
        } else {
            logutil.Infof(c.opts.Logger, "removed voter: id=%s", id)
        }
    }
}

func (c *Cluster) handleLeave(ctx context.Context, req transport.LeaveRequest) (transport.LeaveResponse, error) {
    ctx, end := tracing.StartSpan(ctx, "cluster.handleLeave")
    defer end()
    // Only leader processes leave/removal
    if c.cons == nil || !c.cons.IsLeader() {
        logutil.Warnf(c.opts.Logger, "leave rejected (not leader): id=%s", req.ID)
        return transport.LeaveResponse{Accepted: false, Error: "not leader"}, nil
    }
    // Reconfigure raft: remove server
    c.removeServer(req.ID)
    // Apply state removal via Raft FSM (idempotent)
    c.applyRemoveNode(req.ID)
    logutil.Infof(c.opts.Logger, "leave accepted: id=%s", req.ID)
    return transport.LeaveResponse{Accepted: true}, nil
}

// LeaderCh exposes leadership change events if the underlying consensus
// implementation supports it (via consensus.LeaderNotifier). Returns nil when
// unsupported.
func (c *Cluster) LeaderCh() <-chan consensus.LeaderInfo {
    if c.cons == nil { return nil }
    if ln, ok := c.cons.(consensus.LeaderNotifier); ok {
        return ln.LeaderCh()
    }
    return nil
}

// AppWrite executes a write operation via AppHandlers. If this node is not the
// leader, it forwards the request to the leader over the management RPC client.
func (c *Cluster) AppWrite(ctx context.Context, op string, data []byte) ([]byte, error) {
    if c.cons != nil && c.cons.IsLeader() {
        if c.opts.AppHandlers == nil { return nil, errors.New("cluster: no app handlers") }
        return c.opts.AppHandlers.HandleWrite(ctx, op, data)
    }
    if c.rpcC == nil { return nil, errors.New("cluster: no RPC client configured") }
    // Resolve leader management address
    var leaderMgmt string
    if id, _, ok := c.cons.Leader(); ok {
        leaderMgmt = c.lookupMemberAddr(id)
    }
    if leaderMgmt == "" && c.rpcS != nil {
        // Fallback: query our own status endpoint which proxies to leader and returns LeaderAddr
        if data, err := c.rpcC.GetStatus(ctx, c.rpcS.Addr()); err == nil {
            var st ClusterStatus
            if json.Unmarshal(data, &st) == nil && st.LeaderAddr != "" {
                leaderMgmt = st.LeaderAddr
            }
        }
    }
    if leaderMgmt == "" { return nil, ErrNotLeader }
    resp, err := c.rpcC.PostAppWrite(ctx, leaderMgmt, transport.AppWriteRequest{Op: op, Data: data})
    if err != nil { return nil, err }
    if resp.Error != "" { return nil, errors.New(resp.Error) }
    return resp.Data, nil
}

func (c *Cluster) handleAppWrite(ctx context.Context, req transport.AppWriteRequest) (transport.AppWriteResponse, error) {
    // Only leader processes application writes
    if c.cons == nil || !c.cons.IsLeader() {
        return transport.AppWriteResponse{Error: "not leader"}, nil
    }
    if c.opts.AppHandlers == nil {
        return transport.AppWriteResponse{Error: "no app handlers"}, nil
    }
    out, err := c.opts.AppHandlers.HandleWrite(ctx, req.Op, req.Data)
    if err != nil { return transport.AppWriteResponse{Error: err.Error()}, nil }
    return transport.AppWriteResponse{Data: out}, nil
}

// Publish distributes a sync message (topic+data) to all nodes. Only the leader
// performs the fanout; followers return ErrNotLeader. Best-effort: errors to
// individual nodes are logged and ignored.
func (c *Cluster) Publish(ctx context.Context, topic string, data []byte) error {
    if c.cons == nil || !c.cons.IsLeader() {
        return ErrNotLeader
    }
    // Backpressure on window: block if (seq - minAck) > window
    window := c.opts.ReplWindow
    if window <= 0 { window = 1024 }
    for {
        // read minAck from server
        var minAck uint64
        if rs, ok := c.rpcS.(interface{ AckedAll() map[string]uint64 }); ok {
            if a := rs.AckedAll(); a != nil {
                minAck = a[topic]
            }
        }
        c.rep.mu.Lock()
        cur := c.rep.seq[topic]
        c.rep.mu.Unlock()
        if int(cur-minAck) < window { break }
        // wait/retry or ctx cancel
        select {
        case <-ctx.Done(): return ctx.Err()
        case <-time.After(100 * time.Millisecond):
        }
    }
    if c.opts.AppHandlers != nil {
        // Apply locally first
        _ = c.opts.AppHandlers.HandleSync(ctx, topic, data)
    }
    // Prefer streaming broadcast when available
    // assign sequence
    c.rep.mu.Lock()
    c.rep.seq[topic]++
    seq := c.rep.seq[topic]
    if c.rep.buf[topic] == nil { c.rep.buf[topic] = make(map[uint64][]byte) }
    // keep a copy for retry
    c.rep.buf[topic][seq] = append([]byte(nil), data...)
    // optional: persist to disk
    if dir := c.opts.ReplBufferDir; dir != "" {
        _ = os.WriteFile(c.bufFilePath(dir, topic, seq), data, 0o644)
    }
    c.rep.mu.Unlock()
    // metrics publish
    obsmetrics.ReplicationPublishedTotal.WithLabelValues(topic).Inc()
    obsmetrics.ReplicationSeq.WithLabelValues(topic).Set(float64(seq))
    if b, ok := c.rpcS.(interface{ BroadcastWithSeq(string, []byte, uint64) int }); ok {
        if n := b.BroadcastWithSeq(topic, data, seq); n > 0 {
            return nil
        }
    }
    if c.rpcC == nil || c.mem == nil { return nil }
    for _, m := range c.mem.Members() {
        if m.ID == string(c.opts.NodeID) { continue }
        addr := m.Addr
        if m.Meta != nil { if mg := m.Meta["mgmt"]; mg != "" { addr = mg } }
        // fire-and-forget with short timeout
        go func(a string) {
            cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
            defer cancel()
            _, _ = c.rpcC.PostAppSync(cctx, a, transport.AppSyncRequest{Topic: topic, Data: data})
        }(addr)
    }
    return nil
}

func (c *Cluster) handleAppSync(ctx context.Context, req transport.AppSyncRequest) (transport.AppSyncResponse, error) {
    if c.opts.AppHandlers == nil { return transport.AppSyncResponse{}, nil }
    if err := c.opts.AppHandlers.HandleSync(ctx, req.Topic, req.Data); err != nil {
        return transport.AppSyncResponse{Error: err.Error()}, nil
    }
    return transport.AppSyncResponse{}, nil
}

func (c *Cluster) replicationSubscribeLoop(ctx context.Context) {
    rc, ok := c.rpcC.(interface{ Subscribe(context.Context, string, string, func(string, []byte)) error })
    if !ok { return }
    for {
        if ctx.Err() != nil { return }
        // Resolve leader
        var leaderMgmt string
        if c.cons != nil {
            if id, _, ok := c.cons.Leader(); ok {
                if id != string(c.opts.NodeID) { leaderMgmt = c.lookupMemberAddr(id) }
            }
        }
        if leaderMgmt == "" {
            time.Sleep(500 * time.Millisecond)
            continue
        }
        _ = rc.Subscribe(ctx, leaderMgmt, string(c.opts.NodeID), func(topic string, data []byte) {
            if c.opts.AppHandlers != nil { _ = c.opts.AppHandlers.HandleSync(ctx, topic, data) }
        })
        // backoff before retrying
        select{
        case <-ctx.Done(): return
        case <-time.After(500 * time.Millisecond):
        }
    }
}

// replicationRetryLoop periodically resends unacked messages based on leader-side ack progress.
// This is a best-effort at-least-once delivery with global (not per-node) ack.
func (c *Cluster) replicationRetryLoop(ctx context.Context) {
    ticker := time.NewTicker(1500 * time.Millisecond)
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            if c.cons == nil || !c.cons.IsLeader() { continue }
            // read acked per topic from server (if available)
            var acked map[string]uint64
            if rs, ok := c.rpcS.(interface{ AckedAll() map[string]uint64 }); ok {
                acked = rs.AckedAll()
            } else {
                acked = map[string]uint64{}
            }
            // resend missing seqs and cleanup acknowledged ones
            c.rep.mu.Lock()
            for topic, seqs := range c.rep.buf {
                a := acked[topic]
                // cleanup acknowledged
                for s := range seqs {
                    if s <= a {
                        delete(seqs, s)
                        if dir := c.opts.ReplBufferDir; dir != "" {
                            _ = os.Remove(c.bufFilePath(dir, topic, s))
                        }
                    }
                }
                // determine max seq
                max := c.rep.seq[topic]
                // resend from a+1..max
                if b, ok := c.rpcS.(interface{ BroadcastWithSeq(string, []byte, uint64) int }); ok {
                    for s := a + 1; s <= max; s++ {
                        if data, ok := seqs[s]; ok {
                            _ = b.BroadcastWithSeq(topic, data, s)
                        }
                    }
                }
            }
            c.rep.mu.Unlock()
        }
    }
}

// electionWatchLoop emits election start/end events based on leader availability
// when the underlying consensus does not expose explicit election lifecycle.
func (c *Cluster) electionWatchLoop(ctx context.Context) {
    ticker := time.NewTicker(200 * time.Millisecond)
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            if c.cons == nil { continue }
            _, _, ok := c.cons.Leader()
            c.el.mu.Lock()
            had := c.el.hadLeader
            inProg := c.el.inProgress
            if had && !ok && !inProg {
                // leader lost → election start
                c.el.inProgress = true
                c.el.mu.Unlock()
                c.eb.publish(Event{Type: EventElectionStart, At: time.Now()})
                if c.opts.OnElectionStart != nil { c.opts.OnElectionStart() }
            } else {
                c.el.mu.Unlock()
            }
        }
    }
}

func (c *Cluster) sanitizeTopic(topic string) string {
    // very simple: replace path separators and spaces
    r := strings.NewReplacer("/", "_", "\\", "_", " ", "_")
    return r.Replace(topic)
}

func (c *Cluster) bufFilePath(dir, topic string, seq uint64) string {
    st := c.sanitizeTopic(topic)
    return filepath.Join(dir, fmt.Sprintf("%s_%s.bin", st, strconv.FormatUint(seq, 10)))
}

func (c *Cluster) restoreBufferFromDisk(dir string) {
    entries, err := os.ReadDir(dir)
    if err != nil { return }
    for _, e := range entries {
        if e.IsDir() { continue }
        name := e.Name()
        // expected: <sanitizedTopic>_<seq>.bin
        if !strings.HasSuffix(name, ".bin") { continue }
        base := strings.TrimSuffix(name, ".bin")
        us := strings.LastIndex(base, "_")
        if us <= 0 { continue }
        st := base[:us]
        seqStr := base[us+1:]
        seq, err := strconv.ParseUint(seqStr, 10, 64)
        if err != nil { continue }
        data, err := os.ReadFile(filepath.Join(dir, name))
        if err != nil { continue }
        topic := st // sanitized; acceptable for resend and cleanup; original topic only used as label
        if c.rep.buf[topic] == nil { c.rep.buf[topic] = make(map[uint64][]byte) }
        c.rep.buf[topic][seq] = data
        if seq > c.rep.seq[topic] { c.rep.seq[topic] = seq }
    }
}
