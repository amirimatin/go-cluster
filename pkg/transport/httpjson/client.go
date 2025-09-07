package httpjson

import (
    "crypto/tls"
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "time"

    "github.com/amirimatin/go-cluster/pkg/transport"
)

// Client is a thin HTTP client for the management API. It supports optional
// TLS configuration and simple retry with backoff for robustness.
type Client struct {
    httpc *http.Client
    transport *http.Transport
    isTLS bool
}

// NewClient constructs a new Client with the given timeout.
func NewClient(timeout time.Duration) *Client {
    if timeout <= 0 { timeout = 3 * time.Second }
    tr := &http.Transport{}
    return &Client{httpc: &http.Client{Timeout: timeout, Transport: tr}, transport: tr}
}

// UseTLS sets the TLS config for the underlying HTTP client and switches the
// request scheme to https.
func (c *Client) UseTLS(cfg *tls.Config) *Client {
    if c.transport != nil { c.transport.TLSClientConfig = cfg }
    c.isTLS = cfg != nil
    return c
}

func (c *Client) GetStatus(ctx context.Context, addr string) ([]byte, error) {
    // addr expected as host:port from membership; prefix scheme based on TLS
    scheme := "http"
    if c.isTLS { scheme = "https" }
    url := fmt.Sprintf("%s://%s/status", scheme, addr)
    req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
    if err != nil { return nil, err }
    var lastErr error
    for attempt := 0; attempt < 3; attempt++ {
        resp, err := c.httpc.Do(req)
        if err != nil {
            lastErr = err
        } else {
            defer resp.Body.Close()
            if resp.StatusCode != http.StatusOK {
                b, _ := io.ReadAll(resp.Body)
                lastErr = fmt.Errorf("status %d: %s", resp.StatusCode, string(b))
            } else {
                return io.ReadAll(resp.Body)
            }
        }
        // backoff unless context is done
        select {
        case <-ctx.Done():
            return nil, ctx.Err()
        case <-time.After(time.Duration(100*(1<<attempt)) * time.Millisecond):
        }
    }
    return nil, lastErr
}

func (c *Client) PostJoin(ctx context.Context, addr string, req transport.JoinRequest) (transport.JoinResponse, error) {
    scheme := "http"
    if c.isTLS { scheme = "https" }
    url := fmt.Sprintf("%s://%s/join", scheme, addr)
    var out transport.JoinResponse
    body, err := json.Marshal(req)
    if err != nil { return out, err }
    httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
    if err != nil { return out, err }
    httpReq.Header.Set("Content-Type", "application/json")
    var lastErr error
    for attempt := 0; attempt < 3; attempt++ {
        resp, err := c.httpc.Do(httpReq)
        if err != nil {
            lastErr = err
        } else {
            func() {
                defer resp.Body.Close()
                b, _ := io.ReadAll(resp.Body)
                _ = json.Unmarshal(b, &out)
                if resp.StatusCode != http.StatusOK {
                    if out.Error != "" {
                        lastErr = fmt.Errorf(out.Error)
                    } else {
                        lastErr = fmt.Errorf("join status %d: %s", resp.StatusCode, string(b))
                    }
                } else {
                    lastErr = nil
                }
            }()
            if lastErr == nil { return out, nil }
        }
        // backoff unless context is done
        select {
        case <-ctx.Done():
            if lastErr == nil { lastErr = ctx.Err() }
            return out, lastErr
        case <-time.After(time.Duration(100*(1<<attempt)) * time.Millisecond):
        }
    }
    return out, lastErr
}

var _ transport.RPCClient = (*Client)(nil)

func (c *Client) PostLeave(ctx context.Context, addr string, req transport.LeaveRequest) (transport.LeaveResponse, error) {
    scheme := "http"
    if c.isTLS { scheme = "https" }
    url := fmt.Sprintf("%s://%s/leave", scheme, addr)
    var out transport.LeaveResponse
    body, err := json.Marshal(req)
    if err != nil { return out, err }
    httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
    if err != nil { return out, err }
    httpReq.Header.Set("Content-Type", "application/json")
    var lastErr error
    for attempt := 0; attempt < 3; attempt++ {
        resp, err := c.httpc.Do(httpReq)
        if err != nil {
            lastErr = err
        } else {
            func() {
                defer resp.Body.Close()
                b, _ := io.ReadAll(resp.Body)
                _ = json.Unmarshal(b, &out)
                if resp.StatusCode != http.StatusOK {
                    if out.Error != "" {
                        lastErr = fmt.Errorf(out.Error)
                    } else {
                        lastErr = fmt.Errorf("leave status %d: %s", resp.StatusCode, string(b))
                    }
                } else {
                    lastErr = nil
                }
            }()
            if lastErr == nil { return out, nil }
        }
        // backoff unless context is done
        select {
        case <-ctx.Done():
            if lastErr == nil { lastErr = ctx.Err() }
            return out, lastErr
        case <-time.After(time.Duration(100*(1<<attempt)) * time.Millisecond):
        }
    }
    return out, lastErr
}

func (c *Client) PostAppWrite(ctx context.Context, addr string, req transport.AppWriteRequest) (transport.AppWriteResponse, error) {
    scheme := "http"
    if c.isTLS { scheme = "https" }
    url := fmt.Sprintf("%s://%s/appwrite", scheme, addr)
    var out transport.AppWriteResponse
    body, err := json.Marshal(req)
    if err != nil { return out, err }
    httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
    if err != nil { return out, err }
    httpReq.Header.Set("Content-Type", "application/json")
    var lastErr error
    for attempt := 0; attempt < 3; attempt++ {
        resp, err := c.httpc.Do(httpReq)
        if err != nil {
            lastErr = err
        } else {
            func() {
                defer resp.Body.Close()
                b, _ := io.ReadAll(resp.Body)
                _ = json.Unmarshal(b, &out)
                if resp.StatusCode != http.StatusOK {
                    if out.Error != "" {
                        lastErr = fmt.Errorf(out.Error)
                    } else {
                        lastErr = fmt.Errorf("appwrite status %d: %s", resp.StatusCode, string(b))
                    }
                } else {
                    lastErr = nil
                }
            }()
            if lastErr == nil { return out, nil }
        }
        select {
        case <-ctx.Done():
            if lastErr == nil { lastErr = ctx.Err() }
            return out, lastErr
        case <-time.After(time.Duration(100*(1<<attempt)) * time.Millisecond):
        }
    }
    return out, lastErr
}

func (c *Client) PostAppSync(ctx context.Context, addr string, req transport.AppSyncRequest) (transport.AppSyncResponse, error) {
    scheme := "http"
    if c.isTLS { scheme = "https" }
    url := fmt.Sprintf("%s://%s/appsync", scheme, addr)
    var out transport.AppSyncResponse
    body, err := json.Marshal(req)
    if err != nil { return out, err }
    httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
    if err != nil { return out, err }
    httpReq.Header.Set("Content-Type", "application/json")
    var lastErr error
    for attempt := 0; attempt < 3; attempt++ {
        resp, err := c.httpc.Do(httpReq)
        if err != nil {
            lastErr = err
        } else {
            func() {
                defer resp.Body.Close()
                b, _ := io.ReadAll(resp.Body)
                _ = json.Unmarshal(b, &out)
                if resp.StatusCode != http.StatusOK {
                    if out.Error != "" { lastErr = fmt.Errorf(out.Error) } else { lastErr = fmt.Errorf("appsync status %d: %s", resp.StatusCode, string(b)) }
                } else { lastErr = nil }
            }()
            if lastErr == nil { return out, nil }
        }
        select {
        case <-ctx.Done():
            if lastErr == nil { lastErr = ctx.Err() }
            return out, lastErr
        case <-time.After(time.Duration(100*(1<<attempt)) * time.Millisecond):
        }
    }
    return out, lastErr
}
