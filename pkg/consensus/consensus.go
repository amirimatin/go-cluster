package consensus

import (
    "context"
    "time"
)

// Command represents a RAFT log command. The semantics of Op/Payload are
// defined by the FSM integration (e.g., membership add/remove).
type Command struct {
    Op      string
    Payload []byte
}

// Consensus is the minimal abstraction over a leader-based consensus engine
// (e.g., RAFT). It exposes leadership, term information and a write path.
type Consensus interface {
    Start(ctx context.Context) error
    Apply(cmd Command, timeout time.Duration) error
    IsLeader() bool
    Leader() (id string, addr string, ok bool)
    Term() uint64
    Stop() error
}
