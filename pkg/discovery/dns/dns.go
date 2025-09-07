package dns

import (
    "context"
    "log"
    "net"
    "sort"
    "strconv"
    "strings"
    "sync"
    "time"

    "github.com/amirimatin/go-cluster/pkg/discovery"
)

// Options configures DNS-based discovery.
type Options struct {
    // Names are SRV records or hostnames to resolve.
    // Examples: "_cluster._tcp.example.com" (SRV) or "node1.example.com" (A/AAAA).
    Names []string

    // Port used when resolving A/AAAA records (no port info in DNS answer).
    Port int

    // Refresh controls cache staleness; if zero, defaults to 5s.
    Refresh time.Duration

    // Resolver optionally overrides the DNS resolver used.
    Resolver *net.Resolver

    // Logger optional.
    Logger *log.Logger
}

type impl struct {
    opts Options
    mu   sync.Mutex
    last time.Time
    cache []string
}

// New returns a DNS-backed discovery that resolves SRV and A/AAAA names
// and caches results for the Refresh duration.
func New(opts Options) discovery.Discovery {
    if opts.Refresh <= 0 { opts.Refresh = 5 * time.Second }
    if opts.Port == 0 { opts.Port = 7946 }
    return &impl{opts: opts}
}

func (d *impl) Seeds() []string {
    d.mu.Lock()
    defer d.mu.Unlock()
    if time.Since(d.last) < d.opts.Refresh && len(d.cache) > 0 {
        return append([]string(nil), d.cache...)
    }
    res := d.resolveAll(context.Background())
    d.cache = res
    d.last = time.Now()
    return append([]string(nil), d.cache...)
}

func (d *impl) resolveAll(ctx context.Context) []string {
    seen := make(map[string]struct{})
    var out []string
    for _, name := range d.opts.Names {
        name = strings.TrimSpace(name)
        if name == "" { continue }
        // If already host:port, take as-is
        if strings.Contains(name, ":") && !strings.HasPrefix(name, "_") {
            if _, ok := seen[name]; !ok { out = append(out, name); seen[name] = struct{}{} }
            continue
        }
        // Try SRV first if pattern matches
        if strings.HasPrefix(name, "_") && strings.Contains(name, "._") {
            if recs := d.lookupSRV(ctx, name); len(recs) > 0 {
                for _, hp := range recs { if _, ok := seen[hp]; !ok { out = append(out, hp); seen[hp] = struct{}{} } }
                continue
            }
        }
        // Fallback to A/AAAA
        for _, hp := range d.lookupHost(ctx, name, d.opts.Port) {
            if _, ok := seen[hp]; !ok { out = append(out, hp); seen[hp] = struct{}{} }
        }
    }
    sort.Strings(out)
    return out
}

func (d *impl) lookupSRV(ctx context.Context, fqdn string) []string {
    svc, proto, domain := parseSRVName(fqdn)
    if svc == "" || proto == "" || domain == "" { return nil }
    res := d.opts.Resolver
    if res == nil { res = net.DefaultResolver }
    _, addrs, err := res.LookupSRV(ctx, svc, proto, domain)
    if err != nil { return nil }
    var out []string
    for _, a := range addrs {
        host := strings.TrimSuffix(a.Target, ".")
        hp := net.JoinHostPort(host, strconv.Itoa(int(a.Port)))
        out = append(out, hp)
    }
    return out
}

func (d *impl) lookupHost(ctx context.Context, host string, port int) []string {
    res := d.opts.Resolver
    if res == nil { res = net.DefaultResolver }
    ips, err := res.LookupHost(ctx, host)
    if err != nil { return nil }
    out := make([]string, 0, len(ips))
    for _, ip := range ips {
        out = append(out, net.JoinHostPort(ip, strconv.Itoa(port)))
    }
    return out
}

func parseSRVName(fqdn string) (service, proto, name string) {
    // Expect pattern: _service._proto.name
    parts := strings.SplitN(fqdn, ".", 3)
    if len(parts) < 3 { return "", "", "" }
    s := strings.TrimPrefix(parts[0], "_")
    p := strings.TrimPrefix(parts[1], "_")
    n := parts[2]
    return s, p, n
}

// no extra helpers
