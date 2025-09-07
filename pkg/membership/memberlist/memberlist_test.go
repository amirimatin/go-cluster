package memberlist

import (
    "context"
    "log"
    "net"
    "testing"
    "time"

    base "github.com/amirimatin/go-cluster/pkg/membership"
)

func freePort(t *testing.T) int {
    t.Helper()
    a, err := net.ListenPacket("udp", "127.0.0.1:0")
    if err != nil { t.Fatalf("freePort: %v", err) }
    defer a.Close()
    udpAddr := a.LocalAddr().(*net.UDPAddr)
    return udpAddr.Port
}

func TestMemberlist_StartLocal(t *testing.T) {
    p := freePort(t)
    addr := net.JoinHostPort("127.0.0.1", itoa(p))
    m, err := New(Options{NodeID: "t1", Bind: addr, Advertise: addr, Logger: log.Default(), ProbeInterval: 100 * time.Millisecond})
    if err != nil { t.Fatalf("new: %v", err) }
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    if err := m.Start(ctx); err != nil { t.Fatalf("start: %v", err) }
    defer m.Stop()

    if got := m.Local().ID; got != "t1" { t.Fatalf("local id = %q, want t1", got) }

    if hr, ok := m.(base.HealthReporter); ok {
        if s := hr.HealthScore(); s < -1 { t.Fatalf("unexpected health score: %d", s) }
    } else {
        t.Fatalf("impl does not implement HealthReporter")
    }
}

func TestMemberlist_MultiNodeJoinLeave(t *testing.T) {
    ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
    defer cancel()

    // Start first node with random port
    n1, addr1 := startNode(t, ctx, "n1")
    defer n1.Stop()

    // Start second and third nodes and join to first
    n2, _ := startNode(t, ctx, "n2")
    defer n2.Stop()
    if err := n2.Join([]string{addr1}); err != nil { t.Fatalf("n2 join: %v", err) }

    n3, _ := startNode(t, ctx, "n3")
    defer n3.Stop()
    if err := n3.Join([]string{addr1}); err != nil { t.Fatalf("n3 join: %v", err) }

    // Await convergence to 3 members on each node
    awaitMembers(t, n1, 3, 5*time.Second)
    awaitMembers(t, n2, 3, 5*time.Second)
    awaitMembers(t, n3, 3, 5*time.Second)

    // Now make n2 leave and ensure others see 2 members
    _ = n2.Leave()
    _ = n2.Stop()

    awaitMembers(t, n1, 2, 5*time.Second)
    awaitMembers(t, n3, 2, 5*time.Second)
}

func startNode(t *testing.T, ctx context.Context, id string) (*impl, string) {
    t.Helper()
    addr := "127.0.0.1:0"
    m, err := New(Options{NodeID: id, Bind: addr, Advertise: "", Logger: log.Default(), ProbeInterval: 100 * time.Millisecond, SuspicionMult: 2})
    if err != nil { t.Fatalf("new %s: %v", id, err) }
    if err := m.Start(ctx); err != nil { t.Fatalf("start %s: %v", id, err) }
    // Determine real address after start
    la := m.Local().Addr
    if la == "" { t.Fatalf("local addr empty for %s", id) }
    return m.(*impl), la
}

func awaitMembers(t *testing.T, m base.Membership, want int, timeout time.Duration) {
    t.Helper()
    deadline := time.Now().Add(timeout)
    for {
        got := m.Members()
        if len(got) == want { return }
        if time.Now().After(deadline) {
            t.Fatalf("members timeout: got=%d want=%d list=%v", len(got), want, got)
        }
        time.Sleep(100 * time.Millisecond)
    }
}

// itoa converts an int to string without fmt to keep test lean.
func itoa(i int) string {
    if i == 0 { return "0" }
    sign := ""
    if i < 0 { sign = "-"; i = -i }
    var b [20]byte
    pos := len(b)
    for i > 0 {
        pos--
        b[pos] = byte('0' + i%10)
        i /= 10
    }
    return sign + string(b[pos:])
}
