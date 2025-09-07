package state

import "github.com/amirimatin/go-cluster/pkg/membership"

type MembershipState interface {
    ApplyAddNode(n membership.MemberInfo) error
    ApplyRemoveNode(nodeID string) error
    Snapshot() ([]byte, error)
    Restore(buf []byte) error
}

