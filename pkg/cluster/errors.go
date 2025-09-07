package cluster

import "errors"

var (
    ErrNotLeader       = errors.New("cluster: not leader")
    ErrAlreadyMember   = errors.New("cluster: already a member")
    ErrNoQuorum        = errors.New("cluster: no quorum")
    ErrVersionMismatch = errors.New("cluster: version mismatch")
    ErrUnreachable     = errors.New("cluster: unreachable")
)

