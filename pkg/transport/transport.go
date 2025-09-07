package transport

// Transport abstracts the cluster-internal transport layer sufficiently to
// expose the local advertised address (e.g., RAFT bind address). Higher-level
// management RPC is provided via RPCServer/RPCClient.
type Transport interface {
    // Addr returns the local bind/advertise address if applicable.
    Addr() string
}
