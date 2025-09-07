package cli

import (
    "context"
    "crypto/tls"
    "encoding/json"
    "fmt"
    "log"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/spf13/cobra"

    "github.com/amirimatin/go-cluster/pkg/bootstrap"
    tracing "github.com/amirimatin/go-cluster/pkg/observability/tracing"
    tlsx "github.com/amirimatin/go-cluster/pkg/security/tlsconfig"
    "github.com/amirimatin/go-cluster/pkg/transport"
    mgmtgrpc "github.com/amirimatin/go-cluster/pkg/transport/grpc"
    httpjson "github.com/amirimatin/go-cluster/pkg/transport/httpjson"
)

// AddAll attaches cluster subcommands (run/status/join/leave) to the provided root command.
func AddAll(root *cobra.Command) {
    root.AddCommand(NewRunCmd())
    root.AddCommand(NewStatusCmd())
    root.AddCommand(NewJoinCmd())
    root.AddCommand(NewLeaveCmd())
}

// NewClusterCommand returns a parent command "cluster" containing run/status/join/leave as subcommands.
func NewClusterCommand() *cobra.Command {
    parent := &cobra.Command{Use: "cluster", Short: "cluster management commands"}
    parent.AddCommand(NewRunCmd())
    parent.AddCommand(NewStatusCmd())
    parent.AddCommand(NewJoinCmd())
    parent.AddCommand(NewLeaveCmd())
    return parent
}

// NewRunCmd returns the "run" command used to start a cluster node.
func NewRunCmd() *cobra.Command {
    var (
        id, raftAddr, memBind, memAdv, joinCSV, mgmtAddr, mgmtProto, discoveryKind string
        dnsNames, filePath, fileEnv                                              string
        dnsPort                                                                  int
        discRefresh                                                              time.Duration
        tlsEnable, tlsSkip, traceEnable, doBootstrap                             bool
        tlsCA, tlsCert, tlsKey, tlsServerName, dataDir                           string
    )
    cmd := &cobra.Command{
        Use:   "run",
        Short: "Run a cluster node",
        RunE: func(cmd *cobra.Command, args []string) error {
            if id == "" { return fmt.Errorf("missing -id") }
            ctx, cancel := signalContext()
            defer cancel()

            if traceEnable {
                shutdown, err := tracing.Setup(true)
                if err != nil {
                    log.Printf("tracing setup error: %v", err)
                } else {
                    defer func() { _ = shutdown(context.Background()) }()
                }
            }

            cfg := bootstrap.Config{
                NodeID:        id,
                RaftAddr:      raftAddr,
                MemBind:       memBind,
                MemAdv:        memAdv,
                MgmtAddr:      mgmtAddr,
                MgmtProto:     mgmtProto,
                DiscoveryKind: discoveryKind,
                SeedsCSV:      joinCSV,
                DNSNamesCSV:   dnsNames,
                DNSPort:       dnsPort,
                DiscRefresh:   discRefresh,
                FilePath:      filePath,
                FileEnv:       fileEnv,
                DataDir:       dataDir,
                Bootstrap:     doBootstrap,
                TLSEnable:     tlsEnable,
                TLSCA:         tlsCA,
                TLSCert:       tlsCert,
                TLSKey:        tlsKey,
                TLSServerName: tlsServerName,
                TLSSkipVerify: tlsSkip,
                Logger:        log.Default(),
            }
            cl, err := bootstrap.Run(ctx, cfg)
            if err != nil { return err }
            defer cl.Close()

            fmt.Println("cluster running. Press Ctrl+C to exit.")
            <-ctx.Done()
            return nil
        },
    }
    cmd.Flags().StringVar(&id, "id", "", "node id (required)")
    cmd.Flags().StringVar(&raftAddr, "raft-addr", ":9520", "raft bind addr (tcp)")
    cmd.Flags().StringVar(&memBind, "mem-bind", ":7946", "membership bind addr (host:port)")
    cmd.Flags().StringVar(&memAdv, "mem-adv", "", "membership advertise addr (host:port, optional)")
    cmd.Flags().StringVar(&joinCSV, "join", "", "comma-separated seed nodes (host:port) — used by discovery=static")
    cmd.Flags().StringVar(&mgmtAddr, "mgmt-addr", ":17946", "management address (tcp), separate from membership port")
    cmd.Flags().StringVar(&mgmtProto, "mgmt-proto", "http", "management RPC protocol: http|grpc")
    cmd.Flags().StringVar(&discoveryKind, "discovery", "static", "discovery backend: static|dns|file")
    cmd.Flags().StringVar(&dnsNames, "dns-names", "", "comma-separated DNS names or SRV records (e.g., _cluster._tcp.example.com)")
    cmd.Flags().IntVar(&dnsPort, "dns-port", 7946, "port used for A/AAAA lookups")
    cmd.Flags().DurationVar(&discRefresh, "disc-refresh", 5*time.Second, "discovery refresh/cache duration")
    cmd.Flags().StringVar(&filePath, "file-path", "", "path or glob to a file with seeds (one per line or CSV)")
    cmd.Flags().StringVar(&fileEnv, "file-env", "", "ENV var name containing CSV seeds; overrides file when set")
    cmd.Flags().BoolVar(&tlsEnable, "tls-enable", false, "enable mTLS for management transport")
    cmd.Flags().StringVar(&tlsCA, "tls-ca", "", "path to CA cert (PEM)")
    cmd.Flags().StringVar(&tlsCert, "tls-cert", "", "path to node certificate (PEM)")
    cmd.Flags().StringVar(&tlsKey, "tls-key", "", "path to node private key (PEM)")
    cmd.Flags().BoolVar(&tlsSkip, "tls-skip-verify", false, "skip server cert verification (DEV ONLY)")
    cmd.Flags().StringVar(&tlsServerName, "tls-server-name", "", "expected server name (for TLS validation)")
    cmd.Flags().BoolVar(&traceEnable, "trace", false, "enable OpenTelemetry stdout tracing (dev)")
    cmd.Flags().BoolVar(&doBootstrap, "bootstrap", false, "bootstrap single-node raft (development)")
    cmd.Flags().StringVar(&dataDir, "data", "", "raft data dir (snapshots)")
    return cmd
}

// NewStatusCmd returns the "status" command.
func NewStatusCmd() *cobra.Command {
    var (
        addr    string
        timeout time.Duration
    )
    cmd := &cobra.Command{
        Use:   "status",
        Short: "Fetch cluster status as JSON",
        RunE: func(cmd *cobra.Command, args []string) error {
            ctx, cancel := context.WithTimeout(context.Background(), timeout)
            defer cancel()
            client := httpjson.NewClient(timeout)
            data, err := client.GetStatus(ctx, addr)
            if err != nil { return fmt.Errorf("status error: %w", err) }
            os.Stdout.Write(data)
            if len(data) == 0 || data[len(data)-1] != '\n' { os.Stdout.Write([]byte("\n")) }
            return nil
        },
    }
    cmd.Flags().StringVar(&addr, "addr", "127.0.0.1:17946", "management HTTP address of a node (host:port)")
    cmd.Flags().DurationVar(&timeout, "timeout", 3*time.Second, "request timeout")
    return cmd
}

// NewJoinCmd returns the "join" command.
func NewJoinCmd() *cobra.Command {
    var (
        id, raftAddr, addr, mgmtProto                 string
        timeout                                        time.Duration
        tlsEnable, tlsSkip                             bool
        tlsCA, tlsCert, tlsKey, tlsServerName         string
    )
    cmd := &cobra.Command{
        Use:   "join",
        Short: "Request to add a node to the cluster",
        RunE: func(cmd *cobra.Command, args []string) error {
            if id == "" || raftAddr == "" { return fmt.Errorf("missing required flags: -id and -raft-addr") }
            var client transport.RPCClient
            var cliTLS *tls.Config
            if tlsEnable {
                topts := tlsx.Options{Enable: true, CAFile: tlsCA, CertFile: tlsCert, KeyFile: tlsKey, InsecureSkipVerify: tlsSkip, ServerName: tlsServerName}
                var err error
                cliTLS, err = topts.Client()
                if err != nil { return fmt.Errorf("tls client config: %w", err) }
            }
            switch mgmtProto {
            case "grpc":
                cli := mgmtgrpc.NewClient(timeout)
                if cliTLS != nil { cli.UseTLS(cliTLS) }
                client = cli
            default:
                cli := httpjson.NewClient(timeout)
                if cliTLS != nil { cli.UseTLS(cliTLS) }
                client = cli
            }
            ctx, cancel := context.WithTimeout(context.Background(), timeout)
            defer cancel()
            resp, err := client.PostJoin(ctx, addr, transport.JoinRequest{ID: id, RaftAddr: raftAddr})
            if err != nil { return fmt.Errorf("join error: %w", err) }
            return json.NewEncoder(os.Stdout).Encode(resp)
        },
    }
    cmd.Flags().StringVar(&id, "id", "", "node id to add (required)")
    cmd.Flags().StringVar(&raftAddr, "raft-addr", "", "node raft address (host:port, required)")
    cmd.Flags().StringVar(&addr, "addr", "127.0.0.1:17946", "management address of a node (host:port)")
    cmd.Flags().StringVar(&mgmtProto, "mgmt-proto", "http", "management RPC protocol: http|grpc")
    cmd.Flags().DurationVar(&timeout, "timeout", 3*time.Second, "request timeout")
    cmd.Flags().BoolVar(&tlsEnable, "tls-enable", false, "enable mTLS for management transport")
    cmd.Flags().StringVar(&tlsCA, "tls-ca", "", "path to CA cert (PEM)")
    cmd.Flags().StringVar(&tlsCert, "tls-cert", "", "path to client certificate (PEM)")
    cmd.Flags().StringVar(&tlsKey, "tls-key", "", "path to client private key (PEM)")
    cmd.Flags().BoolVar(&tlsSkip, "tls-skip-verify", false, "skip server cert verification (DEV ONLY)")
    cmd.Flags().StringVar(&tlsServerName, "tls-server-name", "", "expected server name (for TLS validation)")
    return cmd
}

// NewLeaveCmd returns the "leave" command.
func NewLeaveCmd() *cobra.Command {
    var (
        id, addr, mgmtProto                string
        timeout                             time.Duration
        tlsEnable, tlsSkip                  bool
        tlsCA, tlsCert, tlsKey, tlsServerName string
    )
    cmd := &cobra.Command{
        Use:   "leave",
        Short: "Request to remove a node from the cluster",
        RunE: func(cmd *cobra.Command, args []string) error {
            if id == "" { return fmt.Errorf("missing required flag: -id") }
            var client transport.RPCClient
            var cliTLS *tls.Config
            if tlsEnable {
                topts := tlsx.Options{Enable: true, CAFile: tlsCA, CertFile: tlsCert, KeyFile: tlsKey, InsecureSkipVerify: tlsSkip, ServerName: tlsServerName}
                var err error
                cliTLS, err = topts.Client()
                if err != nil { return fmt.Errorf("tls client config: %w", err) }
            }
            switch mgmtProto {
            case "grpc":
                cli := mgmtgrpc.NewClient(timeout)
                if cliTLS != nil { cli.UseTLS(cliTLS) }
                client = cli
            default:
                cli := httpjson.NewClient(timeout)
                if cliTLS != nil { cli.UseTLS(cliTLS) }
                client = cli
            }
            type leaver interface{ PostLeave(context.Context, string, transport.LeaveRequest) (transport.LeaveResponse, error) }
            lv, ok := client.(leaver)
            if !ok { return fmt.Errorf("leave not supported by transport") }
            ctx, cancel := context.WithTimeout(context.Background(), timeout)
            defer cancel()
            resp, err := lv.PostLeave(ctx, addr, transport.LeaveRequest{ID: id})
            if err != nil { return fmt.Errorf("leave error: %w", err) }
            return json.NewEncoder(os.Stdout).Encode(resp)
        },
    }
    cmd.Flags().StringVar(&id, "id", "", "node id to remove (required)")
    cmd.Flags().StringVar(&addr, "addr", "127.0.0.1:17946", "management address of a node (host:port)")
    cmd.Flags().StringVar(&mgmtProto, "mgmt-proto", "http", "management RPC protocol: http|grpc")
    cmd.Flags().DurationVar(&timeout, "timeout", 3*time.Second, "request timeout")
    cmd.Flags().BoolVar(&tlsEnable, "tls-enable", false, "enable mTLS for management transport")
    cmd.Flags().StringVar(&tlsCA, "tls-ca", "", "path to CA cert (PEM)")
    cmd.Flags().StringVar(&tlsCert, "tls-cert", "", "path to client certificate (PEM)")
    cmd.Flags().StringVar(&tlsKey, "tls-key", "", "path to client private key (PEM)")
    cmd.Flags().BoolVar(&tlsSkip, "tls-skip-verify", false, "skip server cert verification (DEV ONLY)")
    cmd.Flags().StringVar(&tlsServerName, "tls-server-name", "", "expected server name (for TLS validation)")
    return cmd
}

func signalContext() (context.Context, context.CancelFunc) {
    ctx, cancel := context.WithCancel(context.Background())
    go func() {
        ch := make(chan os.Signal, 1)
        signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
        <-ch
        cancel()
    }()
    return ctx, cancel
}
