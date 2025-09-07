package tlsconfig

import (
    "crypto/tls"
    "crypto/x509"
    "errors"
    "os"
    "sync"
    "time"
    "io/ioutil"
)

// Options defines mTLS configuration inputs.
type Options struct {
    Enable             bool
    CAFile             string
    CertFile           string
    KeyFile            string
    InsecureSkipVerify bool
    ServerName         string
}

// Server returns a tls.Config for servers if enabled, otherwise nil.
func (o Options) Server() (*tls.Config, error) {
    if !o.Enable {
        return nil, nil
    }
    if o.CertFile == "" || o.KeyFile == "" {
        return nil, errors.New("tls: server cert/key required when TLS enabled")
    }
    cert, err := tls.LoadX509KeyPair(o.CertFile, o.KeyFile)
    if err != nil { return nil, err }
    cfg := &tls.Config{Certificates: []tls.Certificate{cert}}
    if o.CAFile != "" {
        ca, err := ioutil.ReadFile(o.CAFile)
        if err != nil { return nil, err }
        pool := x509.NewCertPool()
        pool.AppendCertsFromPEM(ca)
        cfg.ClientCAs = pool
        cfg.ClientAuth = tls.RequireAndVerifyClientCert
    }
    return cfg, nil
}

// Client returns a tls.Config for clients if enabled, otherwise nil.
func (o Options) Client() (*tls.Config, error) {
    if !o.Enable {
        return nil, nil
    }
    cfg := &tls.Config{InsecureSkipVerify: o.InsecureSkipVerify} //nolint:gosec
    if o.ServerName != "" { cfg.ServerName = o.ServerName }
    if o.CAFile != "" {
        ca, err := ioutil.ReadFile(o.CAFile)
        if err != nil { return nil, err }
        pool := x509.NewCertPool()
        pool.AppendCertsFromPEM(ca)
        cfg.RootCAs = pool
    }
    if o.CertFile != "" && o.KeyFile != "" {
        cert, err := tls.LoadX509KeyPair(o.CertFile, o.KeyFile)
        if err != nil { return nil, err }
        cfg.Certificates = []tls.Certificate{cert}
    }
    return cfg, nil
}

// ServerHotReload returns a server tls.Config that reloads the certificate
// from disk periodically (lazy, on handshake) to support manual rotation
// without restarting the process. CA pool is loaded once.
func (o Options) ServerHotReload() (*tls.Config, error) {
    if !o.Enable {
        return nil, nil
    }
    if o.CertFile == "" || o.KeyFile == "" {
        return nil, errors.New("tls: server cert/key required when TLS enabled")
    }
    // Prepare static CA pool (optional)
    cfg := &tls.Config{}
    if o.CAFile != "" {
        ca, err := ioutil.ReadFile(o.CAFile)
        if err != nil { return nil, err }
        pool := x509.NewCertPool()
        pool.AppendCertsFromPEM(ca)
        cfg.ClientCAs = pool
        cfg.ClientAuth = tls.RequireAndVerifyClientCert
    }
    var (
        mu       sync.RWMutex
        cached   *tls.Certificate
        lastLoad time.Time
    )
    load := func() (*tls.Certificate, error) {
        mu.RLock()
        if cached != nil && time.Since(lastLoad) < 10*time.Second { // ttl
            c := *cached
            mu.RUnlock()
            return &c, nil
        }
        mu.RUnlock()
        cert, err := tls.LoadX509KeyPair(o.CertFile, o.KeyFile)
        if err != nil { return nil, err }
        mu.Lock()
        cached = &cert
        lastLoad = time.Now()
        mu.Unlock()
        return &cert, nil
    }
    cfg.GetCertificate = func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
        return load()
    }
    return cfg, nil
}

// ClientHotReload returns a client tls.Config that reloads the client
// certificate from disk on demand. CA roots are loaded once.
func (o Options) ClientHotReload() (*tls.Config, error) {
    if !o.Enable { return nil, nil }
    cfg := &tls.Config{InsecureSkipVerify: o.InsecureSkipVerify}
    if o.ServerName != "" { cfg.ServerName = o.ServerName }
    if o.CAFile != "" {
        ca, err := os.ReadFile(o.CAFile)
        if err != nil { return nil, err }
        pool := x509.NewCertPool()
        pool.AppendCertsFromPEM(ca)
        cfg.RootCAs = pool
    }
    var (
        mu       sync.RWMutex
        cached   *tls.Certificate
        lastLoad time.Time
    )
    load := func() (*tls.Certificate, error) {
        if o.CertFile == "" || o.KeyFile == "" {
            return nil, nil
        }
        mu.RLock()
        if cached != nil && time.Since(lastLoad) < 10*time.Second {
            c := *cached
            mu.RUnlock()
            return &c, nil
        }
        mu.RUnlock()
        cert, err := tls.LoadX509KeyPair(o.CertFile, o.KeyFile)
        if err != nil { return nil, err }
        mu.Lock()
        cached = &cert
        lastLoad = time.Now()
        mu.Unlock()
        return &cert, nil
    }
    cfg.GetClientCertificate = func(info *tls.CertificateRequestInfo) (*tls.Certificate, error) {
        return load()
    }
    return cfg, nil
}
