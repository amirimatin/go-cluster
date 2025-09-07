package cluster

import (
    "github.com/amirimatin/go-cluster/pkg/membership"
)

// ClusterStatus is a high-level, JSON-serializable snapshot of the cluster
// suitable for external status endpoints and tooling. It aligns with the
// contract described in go-cluster.md §11.
type ClusterStatus struct {
    // Healthy indicates whether a leader is known and basic subsystems are running.
    Healthy    bool
    // Term is the current RAFT term as observed by this node.
    Term       uint64
    // LeaderID is the identifier of the current leader, if any.
    LeaderID   string
    // LeaderAddr is the management address of the current leader, if known.
    LeaderAddr string
    // Members lists the membership view (gossip) including node IDs and addresses.
    Members    []membership.MemberInfo
    // Warnings contains any non-fatal observations (e.g., degraded states).
    Warnings   []string
}
