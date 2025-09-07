package static

import "testing"

func TestParse(t *testing.T) {
    cases := []struct{
        in   string
        want []string
    }{
        {"", nil},
        {"a:1", []string{"a:1"}},
        {" a:1 , b:2 ", []string{"a:1","b:2"}},
        {",,a:1, ,b:2,", []string{"a:1","b:2"}},
    }
    for _, c := range cases {
        got := Parse(c.in)
        if len(got) != len(c.want) {
            t.Fatalf("len mismatch for %q: got %d want %d", c.in, len(got), len(c.want))
        }
        for i := range got {
            if got[i] != c.want[i] {
                t.Fatalf("[%q] item %d: got %q want %q", c.in, i, got[i], c.want[i])
            }
        }
    }
}

func TestNew(t *testing.T) {
    d := New(" a:1 ", "", "b:2")
    got := d.Seeds()
    if len(got) != 2 || got[0] != "a:1" || got[1] != "b:2" {
        t.Fatalf("unexpected seeds: %#v", got)
    }
    // Ensure returned slice is a copy
    got[0] = "x"
    got2 := d.Seeds()
    if got2[0] != "a:1" {
        t.Fatalf("expected defensive copy, got %#v", got2)
    }
}

