package grpc

import (
    "crypto/tls"
    "context"
    "net"
    "time"

    "google.golang.org/grpc"
    "google.golang.org/grpc/health"
    healthpb "google.golang.org/grpc/health/grpc_health_v1"
    "google.golang.org/grpc/credentials"
    "google.golang.org/grpc/keepalive"

    "github.com/amirimatin/go-cluster/pkg/transport"
    "github.com/amirimatin/go-cluster/pkg/observability/tracing"
    obsmetrics "github.com/amirimatin/go-cluster/pkg/observability/metrics"
)

// Server implements transport.RPCServer over gRPC using a JSON codec.
type Server struct {
    bind   string
    lis    net.Listener
    srv    *grpc.Server
    tlsCfg *tls.Config
    // replication subscriptions (leader side)
    reps struct{
        subs map[*repSub]struct{}
        seq  map[string]uint64
        ack  map[string]map[string]uint64
    }
}

func NewServer(bind string) *Server { return &Server{bind: bind} }

// UseTLS enables TLS for the gRPC server using the provided config.
func (s *Server) UseTLS(cfg *tls.Config) *Server { s.tlsCfg = cfg; return s }

// internal request/response types used over gRPC JSON codec
type empty struct{}
type statusBlob struct{ Data []byte `json:"data"` }

// managementServer defines the methods we expose.
type managementServer interface{
    GetStatus(ctx context.Context, in *empty) (*statusBlob, error)
    Join(ctx context.Context, in *transport.JoinRequest) (*transport.JoinResponse, error)
    Leave(ctx context.Context, in *transport.LeaveRequest) (*transport.LeaveResponse, error)
    AppWrite(ctx context.Context, in *transport.AppWriteRequest) (*transport.AppWriteResponse, error)
    AppSync(ctx context.Context, in *transport.AppSyncRequest) (*transport.AppSyncResponse, error)
}

type mgmtImpl struct{ status transport.StatusFunc; join transport.JoinFunc; leave transport.LeaveFunc; appWrite transport.AppWriteFunc; appSync transport.AppSyncFunc }

func (m *mgmtImpl) GetStatus(ctx context.Context, _ *empty) (*statusBlob, error) {
    ctx, end := tracing.StartSpan(ctx, "grpc.status")
    defer end()
    b, err := m.status(ctx)
    if err != nil { return nil, err }
    return &statusBlob{Data: b}, nil
}
func (m *mgmtImpl) Join(ctx context.Context, in *transport.JoinRequest) (*transport.JoinResponse, error) {
    if in == nil { in = &transport.JoinRequest{} }
    ctx, end := tracing.StartSpan(ctx, "grpc.join")
    defer end()
    out, err := m.join(ctx, *in)
    if err != nil { return &transport.JoinResponse{Accepted:false, Error: err.Error()}, nil }
    return &out, nil
}

func (m *mgmtImpl) Leave(ctx context.Context, in *transport.LeaveRequest) (*transport.LeaveResponse, error) {
    if in == nil { in = &transport.LeaveRequest{} }
    if m.leave == nil { return &transport.LeaveResponse{Accepted:false, Error:"leave not supported"}, nil }
    ctx, end := tracing.StartSpan(ctx, "grpc.leave")
    defer end()
    out, err := m.leave(ctx, *in)
    if err != nil { return &transport.LeaveResponse{Accepted:false, Error: err.Error()}, nil }
    return &out, nil
}

func (m *mgmtImpl) AppWrite(ctx context.Context, in *transport.AppWriteRequest) (*transport.AppWriteResponse, error) {
    if in == nil { in = &transport.AppWriteRequest{} }
    if m.appWrite == nil { return &transport.AppWriteResponse{Error: "not implemented"}, nil }
    out, err := m.appWrite(ctx, *in)
    if err != nil { return &transport.AppWriteResponse{Error: err.Error()}, nil }
    return &out, nil
}

func (m *mgmtImpl) AppSync(ctx context.Context, in *transport.AppSyncRequest) (*transport.AppSyncResponse, error) {
    if in == nil { in = &transport.AppSyncRequest{} }
    if m.appSync == nil { return &transport.AppSyncResponse{Error: "not implemented"}, nil }
    out, err := m.appSync(ctx, *in)
    if err != nil { return &transport.AppSyncResponse{Error: err.Error()}, nil }
    return &out, nil
}

// Service descriptor and handlers (hand-written, no codegen required)
var _Management_serviceDesc = grpc.ServiceDesc{
    ServiceName: "cluster.v1.Management",
    HandlerType: (*managementServer)(nil),
    Methods: []grpc.MethodDesc{
        { MethodName: "GetStatus", Handler: _Management_GetStatus_Handler },
        { MethodName: "Join", Handler: _Management_Join_Handler },
        { MethodName: "Leave", Handler: _Management_Leave_Handler },
        { MethodName: "AppWrite", Handler: _Management_AppWrite_Handler },
        { MethodName: "AppSync", Handler: _Management_AppSync_Handler },
    },
}

func _Management_GetStatus_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
    in := new(empty)
    if err := dec(in); err != nil { return nil, err }
    if interceptor == nil { return srv.(managementServer).GetStatus(ctx, in) }
    info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/cluster.v1.Management/GetStatus"}
    handler := func(ctx context.Context, req interface{}) (interface{}, error) {
        return srv.(managementServer).GetStatus(ctx, req.(*empty))
    }
    return interceptor(ctx, in, info, handler)
}

func _Management_Join_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
    in := new(transport.JoinRequest)
    if err := dec(in); err != nil { return nil, err }
    if interceptor == nil { return srv.(managementServer).Join(ctx, in) }
    info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/cluster.v1.Management/Join"}
    handler := func(ctx context.Context, req interface{}) (interface{}, error) {
        return srv.(managementServer).Join(ctx, req.(*transport.JoinRequest))
    }
    return interceptor(ctx, in, info, handler)
}

func _Management_Leave_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
    in := new(transport.LeaveRequest)
    if err := dec(in); err != nil { return nil, err }
    if interceptor == nil { return srv.(managementServer).Leave(ctx, in) }
    info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/cluster.v1.Management/Leave"}
    handler := func(ctx context.Context, req interface{}) (interface{}, error) {
        return srv.(managementServer).Leave(ctx, req.(*transport.LeaveRequest))
    }
    return interceptor(ctx, in, info, handler)
}

func _Management_AppWrite_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
    in := new(transport.AppWriteRequest)
    if err := dec(in); err != nil { return nil, err }
    if interceptor == nil { return srv.(managementServer).AppWrite(ctx, in) }
    info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/cluster.v1.Management/AppWrite"}
    handler := func(ctx context.Context, req interface{}) (interface{}, error) {
        return srv.(managementServer).AppWrite(ctx, req.(*transport.AppWriteRequest))
    }
    return interceptor(ctx, in, info, handler)
}

func _Management_AppSync_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
    in := new(transport.AppSyncRequest)
    if err := dec(in); err != nil { return nil, err }
    if interceptor == nil { return srv.(managementServer).AppSync(ctx, in) }
    info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/cluster.v1.Management/AppSync"}
    handler := func(ctx context.Context, req interface{}) (interface{}, error) {
        return srv.(managementServer).AppSync(ctx, req.(*transport.AppSyncRequest))
    }
    return interceptor(ctx, in, info, handler)
}

func (s *Server) Start(ctx context.Context, status transport.StatusFunc, join transport.JoinFunc, leave transport.LeaveFunc, appWrite transport.AppWriteFunc, appSync transport.AppSyncFunc) error {
    lis, err := net.Listen("tcp", s.bind)
    if err != nil { return err }
    s.lis = lis
    // Force JSON codec to avoid requiring protobuf types
    var opts []grpc.ServerOption
    opts = append(opts, grpc.ForceServerCodec(jsonCodec{}))
    // keepalive settings for long-lived streams
    opts = append(opts, grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{MinTime: 5 * time.Second, PermitWithoutStream: true}))
    opts = append(opts, grpc.KeepaliveParams(keepalive.ServerParameters{Time: 30 * time.Second, Timeout: 10 * time.Second}))
    if s.tlsCfg != nil { opts = append(opts, grpc.Creds(credentials.NewTLS(s.tlsCfg))) }
    srv := grpc.NewServer(opts...)
    s.srv = srv
    // Health service (always serving for now)
    healthSrv := health.NewServer()
    healthpb.RegisterHealthServer(srv, healthSrv)
    // Register management service
    srv.RegisterService(&_Management_serviceDesc, &mgmtImpl{status: status, join: join, leave: leave, appWrite: appWrite, appSync: appSync})

    // Register replication streaming service
    s.reps.subs = make(map[*repSub]struct{})
    s.reps.seq = make(map[string]uint64)
    s.reps.ack = make(map[string]map[string]uint64)
    srv.RegisterService(&_Replication_serviceDesc, &replicationImpl{server: s})

    go func() {
        <-ctx.Done()
        // Graceful stop with a small timeout fallback
        ch := make(chan struct{})
        go func() { srv.GracefulStop(); close(ch) }()
        select {
        case <-ch:
        case <-time.After(2*time.Second):
            srv.Stop()
        }
    }()
    go func() { _ = srv.Serve(lis) }()
    return nil
}

func (s *Server) Addr() string { return s.bind }

func (s *Server) Stop(ctx context.Context) error {
    if s.srv == nil { return nil }
    ch := make(chan struct{})
    go func() { s.srv.GracefulStop(); close(ch) }()
    select {
    case <-ch:
    case <-ctx.Done():
        s.srv.Stop()
    }
    s.srv = nil
    if s.lis != nil { _ = s.lis.Close(); s.lis = nil }
    return nil
}

var _ transport.RPCServer = (*Server)(nil)

// --- Replication streaming ---

type repMsg struct{ Topic string `json:"topic"`; Data []byte `json:"data"`; Seq uint64 `json:"seq"` }
type repSubReq struct{ NodeID string `json:"nodeId,omitempty"` }
type repAck struct{ Topic string `json:"topic"`; Seq uint64 `json:"seq"`; NodeID string `json:"nodeId,omitempty"` }

type repSub struct{ ss grpc.ServerStream; nodeID string }

type replicationServer interface{
    Subscribe(*repSubReq, Replication_SubscribeServer) error
    Ack(context.Context, *repAck) (*empty, error)
}

type Replication_SubscribeServer interface{
    Send(*repMsg) error
    grpc.ServerStream
}

type replicationImpl struct{ server *Server }

func (r *replicationImpl) Subscribe(req *repSubReq, stream Replication_SubscribeServer) error {
    sub := &repSub{ss: stream}
    if req != nil { sub.nodeID = req.NodeID }
    r.server.addSub(sub)
    defer r.server.removeSub(sub)
    // Block until client disconnects
    <-stream.Context().Done()
    return nil
}

func (r *replicationImpl) Ack(ctx context.Context, a *repAck) (*empty, error) {
    if a == nil { return &empty{}, nil }
    // update ack metrics and lag = seq - ack
    if r.server != nil {
        if a.Topic != "" && a.NodeID != "" {
            if r.server.reps.ack[a.Topic] == nil { r.server.reps.ack[a.Topic] = make(map[string]uint64) }
            r.server.reps.ack[a.Topic][a.NodeID] = a.Seq
            obsmetrics.ReplicationAckTotal.WithLabelValues(a.Topic).Inc()
            obsmetrics.ReplicationAckPerNodeTotal.WithLabelValues(a.Topic, a.NodeID).Inc()
            // per-topic lag using min ack across nodes
            minAck := uint64(0)
            first := true
            for _, v := range r.server.reps.ack[a.Topic] {
                if first || v < minAck { minAck = v; first = false }
            }
            if s := r.server.reps.seq[a.Topic]; s >= minAck && !first {
                obsmetrics.ReplicationAckSeq.WithLabelValues(a.Topic).Set(float64(minAck))
                obsmetrics.ReplicationLag.WithLabelValues(a.Topic).Set(float64(s - minAck))
                // per-node gauges as well
                obsmetrics.ReplicationAckSeqPerNode.WithLabelValues(a.Topic, a.NodeID).Set(float64(a.Seq))
                obsmetrics.ReplicationLagPerNode.WithLabelValues(a.Topic, a.NodeID).Set(float64(s - a.Seq))
            }
        }
    }
    return &empty{}, nil
}

func (s *Server) addSub(sub *repSub) {
    if s == nil || s.srv == nil { return }
    if s.reps.subs == nil { s.reps.subs = make(map[*repSub]struct{}) }
    s.reps.subs[sub] = struct{}{}
    obsmetrics.ReplicationSubs.Inc()
}

func (s *Server) removeSub(sub *repSub) {
    if s == nil || s.reps.subs == nil { return }
    delete(s.reps.subs, sub)
    obsmetrics.ReplicationSubs.Dec()
}

// Broadcast sends a replication message to all active subscribers. Returns count sent.
func (s *Server) Broadcast(topic string, data []byte) int {
    if s == nil || s.reps.subs == nil { return 0 }
    // compatibility wrapper, no seq
    return s.BroadcastWithSeq(topic, data, 0)
}

// BroadcastWithSeq sends a replication message to all active subscribers and records metrics.
func (s *Server) BroadcastWithSeq(topic string, data []byte, seq uint64) int {
    if s == nil || s.reps.subs == nil { return 0 }
    msg := &repMsg{Topic: topic, Data: data, Seq: seq}
    cnt := 0
    for sub := range s.reps.subs {
        if err := sub.ss.SendMsg(msg); err == nil { cnt++ } else { delete(s.reps.subs, sub) }
    }
    // metrics
    s.reps.seq[topic] = seq
    obsmetrics.ReplicationBroadcastTotal.WithLabelValues(topic).Add(float64(cnt))
    obsmetrics.ReplicationSeq.WithLabelValues(topic).Set(float64(seq))
    // update lag = seq - minAck
    if m := s.minAck(topic); m > 0 && seq >= m {
        obsmetrics.ReplicationLag.WithLabelValues(topic).Set(float64(seq - m))
    }
    return cnt
}

// AckedAll returns a copy of the last acknowledged sequence per topic.
func (s *Server) AckedAll() map[string]uint64 {
    out := make(map[string]uint64, len(s.reps.ack))
    for topic := range s.reps.ack {
        out[topic] = s.minAck(topic)
    }
    return out
}

func (s *Server) minAck(topic string) uint64 {
    var min uint64
    first := true
    for _, v := range s.reps.ack[topic] {
        if first || v < min { min = v; first = false }
    }
    if first { return 0 }
    return min
}

var _Replication_serviceDesc = grpc.ServiceDesc{
    ServiceName: "cluster.v1.Replication",
    HandlerType: (*replicationServer)(nil),
    Streams: []grpc.StreamDesc{{
        StreamName:    "Subscribe",
        ServerStreams: true,
        Handler:       _Replication_Subscribe_Handler,
    }},
    Methods: []grpc.MethodDesc{{
        MethodName: "Ack",
        Handler:    _Replication_Ack_Handler,
    }},
}

func _Replication_Subscribe_Handler(srv interface{}, stream grpc.ServerStream) error {
    m := new(repSubReq)
    if err := stream.RecvMsg(m); err != nil { return err }
    return srv.(replicationServer).Subscribe(m, &replicationSubscribeServer{stream})
}

type replicationSubscribeServer struct{ grpc.ServerStream }
func (x *replicationSubscribeServer) Send(m *repMsg) error { return x.ServerStream.SendMsg(m) }

func _Replication_Ack_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
    in := new(repAck)
    if err := dec(in); err != nil { return nil, err }
    if interceptor == nil { return srv.(replicationServer).Ack(ctx, in) }
    info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/cluster.v1.Replication/Ack"}
    handler := func(ctx context.Context, req interface{}) (interface{}, error) {
        return srv.(replicationServer).Ack(ctx, req.(*repAck))
    }
    return interceptor(ctx, in, info, handler)
}
