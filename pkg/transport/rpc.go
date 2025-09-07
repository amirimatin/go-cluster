package transport

import "context"

// StatusFunc returns a JSON-encoded status payload for management /status.
// Using []byte avoids import cycles on cluster types.
type StatusFunc func(ctx context.Context) ([]byte, error)

// JoinRequest describes a join intent from a node and carries the RAFT address
// that should be added as a voter to the cluster.
type JoinRequest struct {
    ID       string `json:"id"`
    RaftAddr string `json:"raftAddr"`
}

// JoinResponse indicates acceptance and optionally leader address or error.
type JoinResponse struct {
    Accepted bool   `json:"accepted"`
    Leader   string `json:"leader,omitempty"`
    Error    string `json:"error,omitempty"`
}

// JoinFunc handles node join requests (leader-only).
type JoinFunc func(ctx context.Context, req JoinRequest) (JoinResponse, error)

// RPCServer exposes management endpoints (e.g., /status, /join, /leave, appwrite)
// for intra-cluster calls.
type RPCServer interface {
    Start(ctx context.Context, status StatusFunc, join JoinFunc, leave LeaveFunc, appWrite AppWriteFunc, appSync AppSyncFunc) error
    Addr() string
    Stop(ctx context.Context) error
}

// RPCClient performs intra-cluster calls to other nodes using the chosen
// management protocol (HTTP/JSON or gRPC JSON codec).
type RPCClient interface {
    GetStatus(ctx context.Context, addr string) ([]byte, error)
    PostJoin(ctx context.Context, addr string, req JoinRequest) (JoinResponse, error)
    PostLeave(ctx context.Context, addr string, req LeaveRequest) (LeaveResponse, error)
    PostAppWrite(ctx context.Context, addr string, req AppWriteRequest) (AppWriteResponse, error)
    PostAppSync(ctx context.Context, addr string, req AppSyncRequest) (AppSyncResponse, error)
}

// LeaveRequest requests removal of a node from the cluster.
type LeaveRequest struct {
    ID string `json:"id"`
}

// LeaveResponse indicates whether the leave/remove was accepted.
type LeaveResponse struct {
    Accepted bool   `json:"accepted"`
    Error    string `json:"error,omitempty"`
}

// LeaveFunc handles node leave requests (leader-only).
type LeaveFunc func(ctx context.Context, req LeaveRequest) (LeaveResponse, error)

// Application write forwarding
type AppWriteRequest struct {
    Op   string `json:"op"`
    Data []byte `json:"data"`
}

type AppWriteResponse struct {
    Data  []byte `json:"data,omitempty"`
    Error string `json:"error,omitempty"`
}

type AppWriteFunc func(ctx context.Context, req AppWriteRequest) (AppWriteResponse, error)

// Application sync (leader -> nodes)
type AppSyncRequest struct {
    Topic string `json:"topic"`
    Data  []byte `json:"data"`
}

type AppSyncResponse struct {
    Error string `json:"error,omitempty"`
}

type AppSyncFunc func(ctx context.Context, req AppSyncRequest) (AppSyncResponse, error)

// ReplicationClient is an optional client for streaming replication
// subscriptions (gRPC-only). Implementations should use persistent connections
// with keepalive and backoff.
type ReplicationClient interface {
    // Subscribe establishes a long-lived server-stream from addr and invokes
    // onMsg for each incoming replication message. It blocks until the stream
    // ends or ctx is done. Implementations should use keepalive and backoff.
    Subscribe(ctx context.Context, addr string, nodeID string, onMsg func(topic string, data []byte)) error
}
