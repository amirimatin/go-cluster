package cluster

import (
    "errors"
    "log"
    "time"

    "github.com/amirimatin/go-cluster/pkg/discovery"
    "github.com/amirimatin/go-cluster/pkg/consensus"
    "github.com/amirimatin/go-cluster/pkg/transport"
    "github.com/amirimatin/go-cluster/pkg/membership"
)

type NodeID string

// Options carries dependency-injected components and runtime configuration used
// to assemble the cluster facade. Instances are typically produced from
// bootstrap.Config.
type Options struct {
    // NodeID is the unique identifier of this node within the cluster.
    NodeID    NodeID
    // Transport is a minimal transport used to store the local RAFT address.
    Transport transport.Transport
    // Discovery provides seed nodes for membership join.
    Discovery discovery.Discovery
    // Logger is used by cluster to report operational messages.
    Logger    *log.Logger

    // Optional injected consensus engine (e.g., Raft). If nil, leadership info
    // will be unavailable in Status() until wired.
    Consensus consensus.Consensus

    // Membership implementation (required)
    Membership membership.Membership

    // Optional management RPC (for Status proxy)
    RPCServer transport.RPCServer
    RPCClient transport.RPCClient

    // Optional application handlers (SPI)
    AppHandlers AppHandlers

    // Replication reliability tuning
    ReplWindow int           // max (seq - minAck) per topic before backpressure
    ReplRetry  time.Duration // retry interval for resending unacked messages
    ReplBufferDir string     // optional directory for disk-backed buffer (leader)

    // Optional callbacks for app-level hooks
    OnLeaderChange  func(info consensus.LeaderInfo)
    OnElectionStart func()
    OnElectionEnd   func(info consensus.LeaderInfo)
}

// Validate performs a minimal validation of Options. It does not start any
// network activity and is safe to call before New.
func (o Options) Validate() error {
    if o.NodeID == "" {
        return errors.New("cluster: empty NodeID")
    }
    if o.Transport == nil {
        return errors.New("cluster: nil Transport")
    }
    if o.Discovery == nil {
        return errors.New("cluster: nil Discovery")
    }
    if o.Logger == nil {
        return errors.New("cluster: nil Logger")
    }
    if o.Membership == nil {
        return errors.New("cluster: nil Membership")
    }
    return nil
}
