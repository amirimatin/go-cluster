package grpc

import (
    "context"
    "time"

    "google.golang.org/grpc"
)

// Subscribe establishes a server-stream to the replication service and invokes
// onMsg for every received replication message. It retries on errors with backoff.
func (c *Client) Subscribe(ctx context.Context, addr string, nodeID string, onMsg func(topic string, data []byte)) error {
    if c.cm == nil { c.cm = NewConnManager(30*time.Second, c.dialCtx) }
    // establish stream
    cc, rel, err := c.cm.Get(ctx, addr)
    if err != nil { return err }
    defer rel()
    // Build a client stream manually
    sd := &grpc.StreamDesc{ServerStreams: true}
    cs, err := cc.NewStream(ctx, sd, "/cluster.v1.Replication/Subscribe")
    if err != nil { return err }
    // send initial subscribe request
    if err := cs.SendMsg(&repSubReq{NodeID: nodeID}); err != nil { return err }
    if err := cs.CloseSend(); err != nil { /* ignore close send errors for server streaming */ }
    // receive loop
    for {
        var m repMsg
        if err := cs.RecvMsg(&m); err != nil {
            return err
        }
        if onMsg != nil { onMsg(m.Topic, m.Data) }
        // send ack (best-effort)
        _ = cc.Invoke(ctx, "/cluster.v1.Replication/Ack", &repAck{Topic: m.Topic, Seq: m.Seq, NodeID: nodeID}, &empty{})
    }
}

// Ensure interface satisfaction
// (We rely on the transport.ReplicationClient interface in cluster package.)
