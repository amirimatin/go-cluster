package raftcons

import (
    "log"
    "time"
)

// Options configure the Raft-based Consensus implementation.
type Options struct {
    NodeID   string
    Logger   *log.Logger

    // Bootstrap forms a single-node cluster on Start when true.
    Bootstrap bool

    // Timeouts (optional). Zero means defaults.
    HeartbeatTimeout time.Duration
    ElectionTimeout  time.Duration
    CommitTimeout    time.Duration
    ApplyTimeout     time.Duration // client-side apply wait

    // Networking & Storage
    // If BindAddr is non-empty, a TCP transport is used bound to this address
    // (e.g., "127.0.0.1:0"). Otherwise, an in-memory transport is used.
    BindAddr string

    // DataDir selects on-disk stores when non-empty (bolt store for log/stable,
    // file snapshot store). When empty, in-memory stores are used.
    DataDir string

    // SnapshotsRetained controls how many snapshots to retain on disk.
    SnapshotsRetained int
}
