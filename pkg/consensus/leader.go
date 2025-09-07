package consensus

// LeaderInfo describes the current known leader.
type LeaderInfo struct {
    ID   string
    Addr string
    Term uint64
}

// LeaderNotifier is an optional interface that a Consensus implementation may
// provide to notify about leadership changes via an observable channel.
type LeaderNotifier interface {
    // LeaderCh delivers Leadership updates. The channel should be closed when
    // the consensus node stops. Implementations should buffer and coalesce as
    // needed to avoid blocking the consensus internals.
    LeaderCh() <-chan LeaderInfo
}
