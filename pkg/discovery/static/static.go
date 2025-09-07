package static

import (
    "strings"

    "github.com/amirimatin/go-cluster/pkg/discovery"
)

type staticSeeds struct {
    seeds []string
}

func (s *staticSeeds) Seeds() []string { return append([]string(nil), s.seeds...) }

// New returns a Discovery that always returns the given seeds.
func New(seeds ...string) discovery.Discovery {
    cleaned := make([]string, 0, len(seeds))
    for _, v := range seeds {
        v = strings.TrimSpace(v)
        if v != "" {
            cleaned = append(cleaned, v)
        }
    }
    return &staticSeeds{seeds: cleaned}
}

// Parse converts a comma-separated list into []string seeds.
func Parse(csv string) []string {
    if csv == "" {
        return nil
    }
    parts := strings.Split(csv, ",")
    out := make([]string, 0, len(parts))
    for _, p := range parts {
        p = strings.TrimSpace(p)
        if p != "" {
            out = append(out, p)
        }
    }
    return out
}

