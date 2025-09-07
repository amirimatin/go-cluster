package grpc

import (
    "crypto/tls"
    "context"
    "time"

    "google.golang.org/grpc"
    "google.golang.org/grpc/backoff"
    "google.golang.org/grpc/credentials/insecure"
    "google.golang.org/grpc/credentials"
    "google.golang.org/grpc/keepalive"

    "github.com/amirimatin/go-cluster/pkg/transport"
    "fmt"
)

type Client struct {
    timeout time.Duration
    tlsCfg  *tls.Config
    cm      *ConnManager
}

func NewClient(timeout time.Duration) *Client {
    if timeout <= 0 { timeout = 3 * time.Second }
    c := &Client{timeout: timeout}
    // conn manager wired after we have dialer configured (including TLS)
    return c
}

func (c *Client) dialCtx(ctx context.Context, target string) (*grpc.ClientConn, error) {
    // Use JSON codec and set content subtype accordingly.
    opts := []grpc.DialOption{
        grpc.WithDefaultCallOptions(grpc.ForceCodec(jsonCodec{}), grpc.CallContentSubtype("json")),
        grpc.WithConnectParams(grpc.ConnectParams{Backoff: backoff.DefaultConfig, MinConnectTimeout: 500 * time.Millisecond}),
        grpc.WithKeepaliveParams(keepalive.ClientParameters{Time: 20 * time.Second, Timeout: 5 * time.Second, PermitWithoutStream: true}),
        grpc.WithBlock(),
    }
    if c.tlsCfg != nil {
        opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(c.tlsCfg)))
    } else {
        opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
    }
    return grpc.DialContext(
        ctx,
        target,
        opts...,
    )
}

func (c *Client) GetStatus(ctx context.Context, addr string) ([]byte, error) {
    cctx, cancel := context.WithTimeout(ctx, c.timeout)
    defer cancel()
    cc, rel, err := c.getConn(cctx, addr)
    if err != nil { return nil, err }
    defer rel()
    out := new(statusBlob)
    if err := cc.Invoke(cctx, "/cluster.v1.Management/GetStatus", &empty{}, out); err != nil { return nil, err }
    return out.Data, nil
}

func (c *Client) PostJoin(ctx context.Context, addr string, req transport.JoinRequest) (transport.JoinResponse, error) {
    cctx, cancel := context.WithTimeout(ctx, c.timeout)
    defer cancel()
    var resp transport.JoinResponse
    cc, rel, err := c.getConn(cctx, addr)
    if err != nil { return resp, err }
    defer rel()
    if err := cc.Invoke(cctx, "/cluster.v1.Management/Join", &req, &resp); err != nil { return resp, err }
    return resp, nil
}

func (c *Client) PostLeave(ctx context.Context, addr string, req transport.LeaveRequest) (transport.LeaveResponse, error) {
    cctx, cancel := context.WithTimeout(ctx, c.timeout)
    defer cancel()
    var resp transport.LeaveResponse
    cc, rel, err := c.getConn(cctx, addr)
    if err != nil { return resp, err }
    defer rel()
    if err := cc.Invoke(cctx, "/cluster.v1.Management/Leave", &req, &resp); err != nil { return resp, err }
    return resp, nil
}

func (c *Client) PostAppWrite(ctx context.Context, addr string, req transport.AppWriteRequest) (transport.AppWriteResponse, error) {
    cctx, cancel := context.WithTimeout(ctx, c.timeout)
    defer cancel()
    var resp transport.AppWriteResponse
    cc, err := c.dialCtx(cctx, addr)
    if err != nil { return resp, err }
    defer cc.Close()
    if err := cc.Invoke(cctx, "/cluster.v1.Management/AppWrite", &req, &resp); err != nil { return resp, err }
    if resp.Error != "" { return resp, fmt.Errorf(resp.Error) }
    return resp, nil
}

func (c *Client) PostAppSync(ctx context.Context, addr string, req transport.AppSyncRequest) (transport.AppSyncResponse, error) {
    cctx, cancel := context.WithTimeout(ctx, c.timeout)
    defer cancel()
    var resp transport.AppSyncResponse
    cc, err := c.dialCtx(cctx, addr)
    if err != nil { return resp, err }
    defer cc.Close()
    if err := cc.Invoke(cctx, "/cluster.v1.Management/AppSync", &req, &resp); err != nil { return resp, err }
    if resp.Error != "" { return resp, fmt.Errorf(resp.Error) }
    return resp, nil
}

var _ transport.RPCClient = (*Client)(nil)

// UseTLS sets TLS config for the client.
func (c *Client) UseTLS(cfg *tls.Config) *Client { c.tlsCfg = cfg; return c }

// getConn returns a managed connection, creating a manager if absent.
func (c *Client) getConn(ctx context.Context, addr string) (*grpc.ClientConn, func(), error) {
    if c.cm == nil {
        c.cm = NewConnManager(30*time.Second, c.dialCtx)
    }
    return c.cm.Get(ctx, addr)
}
