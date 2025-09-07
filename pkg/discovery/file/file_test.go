package file

import (
    "os"
    "path/filepath"
    "testing"
    "time"
)

func TestEnvOverridesFile(t *testing.T) {
    dir := t.TempDir()
    f := filepath.Join(dir, "seeds.txt")
    if err := os.WriteFile(f, []byte("a:1\n"), 0o644); err != nil { t.Fatal(err) }

    const envName = "TEST_CLUSTER_SEEDS"
    t.Setenv(envName, "x:9,y:8")

    d := New(Options{Path: f, Env: envName, Refresh: 5 * time.Millisecond})
    got := d.Seeds()
    if len(got) != 2 || got[0] != "x:9" || got[1] != "y:8" {
        t.Fatalf("env override failed, got %#v", got)
    }
}

func TestFileReadAndCacheRefresh(t *testing.T) {
    dir := t.TempDir()
    f := filepath.Join(dir, "seeds.txt")
    if err := os.WriteFile(f, []byte("a:1\nb:2\n"), 0o644); err != nil { t.Fatal(err) }

    d := New(Options{Path: f, Refresh: 10 * time.Millisecond})
    got1 := d.Seeds()
    if len(got1) != 2 || got1[0] != "a:1" || got1[1] != "b:2" {
        t.Fatalf("unexpected initial seeds: %#v", got1)
    }

    // Update file and wait for refresh window
    if err := os.WriteFile(f, []byte("b:2\nc:3\n"), 0o644); err != nil { t.Fatal(err) }
    time.Sleep(15 * time.Millisecond)

    got2 := d.Seeds()
    if len(got2) != 2 || got2[0] != "b:2" || got2[1] != "c:3" {
        t.Fatalf("expected refreshed seeds, got %#v", got2)
    }
}

func TestGlobReadsUniqueSorted(t *testing.T) {
    dir := t.TempDir()
    f1 := filepath.Join(dir, "a.txt")
    f2 := filepath.Join(dir, "b.txt")
    if err := os.WriteFile(f1, []byte("a:1\nb:2\n"), 0o644); err != nil { t.Fatal(err) }
    if err := os.WriteFile(f2, []byte("b:2\nc:3\n"), 0o644); err != nil { t.Fatal(err) }

    pat := filepath.Join(dir, "*.txt")
    d := New(Options{Path: pat, Refresh: 5 * time.Millisecond})
    got := d.Seeds()
    want := []string{"a:1", "b:2", "c:3"}
    if len(got) != len(want) {
        t.Fatalf("len mismatch: got %d want %d (%#v)", len(got), len(want), got)
    }
    for i := range want {
        if got[i] != want[i] {
            t.Fatalf("item %d: got %q want %q (%#v)", i, got[i], want[i], got)
        }
    }
}

