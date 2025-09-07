package dns

import (
    "strings"
    "testing"
    "time"
)

func TestParseSRVName(t *testing.T) {
    s, p, n := parseSRVName("_cluster._tcp.example.com")
    if s != "cluster" || p != "tcp" || n != "example.com" {
        t.Fatalf("parseSRVName failed: got (%q,%q,%q)", s, p, n)
    }
    s, p, n = parseSRVName("bad.srv")
    if s != "" || p != "" || n != "" {
        t.Fatalf("expected empty parts for bad input, got (%q,%q,%q)", s, p, n)
    }
}

func TestPassthroughHostPort(t *testing.T) {
    d := New(Options{Names: []string{"1.2.3.4:7946"}, Refresh: 5 * time.Millisecond})
    got := d.Seeds()
    if len(got) != 1 || got[0] != "1.2.3.4:7946" {
        t.Fatalf("unexpected seeds: %#v", got)
    }
}

func TestLookupHostLocalhost(t *testing.T) {
    d := New(Options{Names: []string{"localhost"}, Port: 12345, Refresh: 5 * time.Millisecond})
    got := d.Seeds()
    if len(got) == 0 {
        t.Fatalf("expected at least one resolved host:port, got %#v", got)
    }
    // Accept IPv4 or IPv6 formatting
    ok := false
    for _, s := range got {
        if strings.HasSuffix(s, ":12345") || strings.HasSuffix(s, "]:12345") {
            ok = true
            break
        }
    }
    if !ok {
        t.Fatalf("expected port suffix in any result, got %#v", got)
    }
}

