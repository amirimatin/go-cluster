package bootstrap

import (
    "context"
    "log"
    "time"

    "github.com/amirimatin/go-cluster/pkg/cluster"
    consraft "github.com/amirimatin/go-cluster/pkg/consensus/raft"
    cns "github.com/amirimatin/go-cluster/pkg/consensus"
    "github.com/amirimatin/go-cluster/pkg/discovery"
    dDNS "github.com/amirimatin/go-cluster/pkg/discovery/dns"
    dFile "github.com/amirimatin/go-cluster/pkg/discovery/file"
    dStatic "github.com/amirimatin/go-cluster/pkg/discovery/static"
    ml "github.com/amirimatin/go-cluster/pkg/membership/memberlist"
    tlsx "github.com/amirimatin/go-cluster/pkg/security/tlsconfig"
    "github.com/amirimatin/go-cluster/pkg/transport"
    mgmtgrpc "github.com/amirimatin/go-cluster/pkg/transport/grpc"
    httpjson "github.com/amirimatin/go-cluster/pkg/transport/httpjson"
    "github.com/amirimatin/go-cluster/pkg/transport/udp"
    "crypto/tls"
)

// Config defines high-level inputs to assemble a cluster node with sensible
// defaults. Applications embed the cluster by providing this structure and
// calling Build/Run.
type Config struct {
    // Identity and addresses
    NodeID   string
    RaftAddr string // e.g., ":9521" or "host:9521"
    MemBind  string // membership bind host:port
    MemAdv   string // optional advertise host:port

    // Management API (status/join/leave/metrics)
    MgmtAddr  string // host:port for management API (HTTP or gRPC)
    MgmtProto string // "http" (default) or "grpc"

    // Discovery settings
    DiscoveryKind string        // "static" (default), "dns", or "file"
    SeedsCSV      string        // used when DiscoveryKind=static
    DNSNamesCSV   string        // used when kind=dns
    DNSPort       int           // used when kind=dns (A/AAAA)
    DiscRefresh   time.Duration // cache/refresh duration for discovery
    FilePath      string        // used when kind=file
    FileEnv       string        // used when kind=file

    // Persistence and bootstrap
    DataDir  string // empty → in-memory
    Bootstrap bool  // single-node bootstrap

    // TLS (optional) for management API
    TLSEnable      bool
    TLSCA          string
    TLSCert        string
    TLSKey         string
    TLSServerName  string
    TLSSkipVerify  bool

    // Logger (optional). If nil, log.Default() is used.
    Logger *log.Logger

    // Application handlers (optional) to handle read/write/sync callbacks.
    AppHandlers cluster.AppHandlers

    // Replication reliability tuning (optional)
    ReplWindow int           // max unacked window per topic before backpressure (default 1024)
    ReplRetry  time.Duration // retry interval (default 1.5s)
    ReplBufferDir string     // optional directory for disk-backed buffer

    // Optional callbacks
    OnLeaderChange  func(info cns.LeaderInfo)
    OnElectionStart func()
    OnElectionEnd   func(info cns.LeaderInfo)
}

// Build assembles a cluster.Cluster from Config without starting it.
func Build(cfg Config) (*cluster.Cluster, error) {
    if cfg.Logger == nil { cfg.Logger = log.Default() }

    // Transport for Raft (stub address carrier)
    tr, err := udp.New(cfg.RaftAddr)
    if err != nil { return nil, err }

    // Discovery backend
    var disc discovery.Discovery
    switch cfg.DiscoveryKind {
    case "dns":
        names := dStatic.Parse(cfg.DNSNamesCSV)
        opts := dDNS.Options{Names: names, Port: cfg.DNSPort}
        if cfg.DiscRefresh > 0 { opts.Refresh = cfg.DiscRefresh }
        disc = dDNS.New(opts)
    case "file":
        opts := dFile.Options{Path: cfg.FilePath, Env: cfg.FileEnv}
        if cfg.DiscRefresh > 0 { opts.Refresh = cfg.DiscRefresh }
        disc = dFile.New(opts)
    default:
        seeds := dStatic.Parse(cfg.SeedsCSV)
        disc = dStatic.New(seeds...)
    }

    // Consensus (Raft)
    cons, err := consraft.New(consraft.Options{NodeID: cfg.NodeID, BindAddr: cfg.RaftAddr, DataDir: cfg.DataDir, Bootstrap: cfg.Bootstrap})
    if err != nil { return nil, err }

    // Membership (memberlist)
    // Pass management address via membership metadata for proxy-to-leader and discovery of mgmt endpoints
    memMeta := map[string]string{}
    if cfg.MgmtAddr != "" { memMeta["mgmt"] = cfg.MgmtAddr }
    mem, err := ml.New(ml.Options{NodeID: cfg.NodeID, Bind: cfg.MemBind, Advertise: cfg.MemAdv, Logger: cfg.Logger, Meta: memMeta})
    if err != nil { return nil, err }

    // Management API
    var srv transport.RPCServer
    var cli transport.RPCClient
    var srvTLS, cliTLS *tls.Config
    if cfg.TLSEnable {
        topts := tlsx.Options{Enable: true, CAFile: cfg.TLSCA, CertFile: cfg.TLSCert, KeyFile: cfg.TLSKey, InsecureSkipVerify: cfg.TLSSkipVerify, ServerName: cfg.TLSServerName}
        // Prefer hot-reload configs to allow manual rotation by replacing files
        if s, err := topts.ServerHotReload(); err == nil { srvTLS = s } else { return nil, err }
        if c, err := topts.ClientHotReload(); err == nil { cliTLS = c } else { return nil, err }
    }
    switch cfg.MgmtProto {
    case "grpc":
        s := mgmtgrpc.NewServer(cfg.MgmtAddr)
        if srvTLS != nil { s.UseTLS(srvTLS) }
        c := mgmtgrpc.NewClient(3 * time.Second)
        if cliTLS != nil { c.UseTLS(cliTLS) }
        srv, cli = s, c
    default:
        s := httpjson.NewServer(cfg.MgmtAddr, cfg.Logger)
        if srvTLS != nil { s.UseTLS(srvTLS) }
        c := httpjson.NewClient(3 * time.Second)
        if cliTLS != nil { c.UseTLS(cliTLS) }
        srv, cli = s, c
    }

    opts := cluster.Options{
        NodeID:     cluster.NodeID(cfg.NodeID),
        Transport:  tr,
        Discovery:  disc,
        Logger:     cfg.Logger,
        Consensus:  cons,
        Membership: mem,
        RPCServer:  srv,
        RPCClient:  cli,
        AppHandlers: cfg.AppHandlers,
        ReplWindow:  cfg.ReplWindow,
        ReplRetry:   cfg.ReplRetry,
        ReplBufferDir: cfg.ReplBufferDir,
        OnLeaderChange:  cfg.OnLeaderChange,
        OnElectionStart: cfg.OnElectionStart,
        OnElectionEnd:   cfg.OnElectionEnd,
    }
    return cluster.New(context.Background(), opts)
}

// Run builds and starts the cluster, returning the instance for lifecycle
// control. The caller is responsible for calling Close() when finished.
func Run(ctx context.Context, cfg Config) (*cluster.Cluster, error) {
    cl, err := Build(cfg)
    if err != nil { return nil, err }
    if err := cl.Start(ctx); err != nil { return nil, err }
    return cl, nil
}
