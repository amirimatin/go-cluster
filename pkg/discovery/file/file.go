package file

import (
    "bufio"
    "os"
    "path/filepath"
    "sort"
    "strings"
    "sync"
    "time"

    "github.com/amirimatin/go-cluster/pkg/discovery"
)

// Options configures file/ENV-based discovery.
type Options struct {
    // Path to a file containing one seed per line or comma-separated list.
    Path string
    // Env overrides file when non-empty.
    Env string
    // Refresh controls cache staleness; if zero, defaults to 5s.
    Refresh time.Duration
}

type impl struct {
    opts Options
    mu   sync.Mutex
    last time.Time
    mtime time.Time
    cache []string
}

func New(opts Options) discovery.Discovery { if opts.Refresh <= 0 { opts.Refresh = 5 * time.Second }; return &impl{opts: opts} }

func (i *impl) Seeds() []string {
    i.mu.Lock(); defer i.mu.Unlock()
    // ENV takes precedence
    if v := strings.TrimSpace(os.Getenv(i.opts.Env)); i.opts.Env != "" && v != "" {
        return parseSeeds(v)
    }
    // File with cache based on mtime and Refresh
    if i.opts.Path == "" {
        return nil
    }
    stat, err := os.Stat(i.opts.Path)
    now := time.Now()
    if err == nil {
        // If file changed or cache is stale, reload
        if stat.ModTime().After(i.mtime) || now.Sub(i.last) >= i.opts.Refresh {
            i.cache = loadFile(i.opts.Path)
            i.last = now
            i.mtime = stat.ModTime()
        }
        return append([]string(nil), i.cache...)
    }
    // try glob
    matches, _ := filepath.Glob(i.opts.Path)
    if len(matches) > 0 {
        var set = make(map[string]struct{})
        for _, m := range matches {
            for _, s := range loadFile(m) { set[s] = struct{}{} }
        }
        var out []string
        for s := range set { out = append(out, s) }
        sort.Strings(out)
        i.cache = out
        i.last = now
        return append([]string(nil), i.cache...)
    }
    return append([]string(nil), i.cache...)
}

func loadFile(path string) []string {
    f, err := os.Open(path)
    if err != nil { return nil }
    defer f.Close()
    var seeds []string
    s := bufio.NewScanner(f)
    for s.Scan() {
        line := strings.TrimSpace(s.Text())
        if line == "" || strings.HasPrefix(line, "#") { continue }
        // allow comma-separated per line
        for _, p := range strings.Split(line, ",") {
            p = strings.TrimSpace(p)
            if p != "" { seeds = append(seeds, p) }
        }
    }
    if err := s.Err(); err != nil { return nil }
    // normalize: de-dup + sort
    set := make(map[string]struct{})
    for _, x := range seeds { set[x] = struct{}{} }
    seeds = seeds[:0]
    for x := range set { seeds = append(seeds, x) }
    sort.Strings(seeds)
    return seeds
}

func parseSeeds(csv string) []string {
    if csv == "" { return nil }
    parts := strings.Split(csv, ",")
    var out []string
    for _, p := range parts {
        p = strings.TrimSpace(p)
        if p != "" { out = append(out, p) }
    }
    sort.Strings(out)
    return out
}

