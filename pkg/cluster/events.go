package cluster

import (
    "context"
    "sync"
    "time"

    "github.com/amirimatin/go-cluster/pkg/consensus"
    "github.com/amirimatin/go-cluster/pkg/membership"
)

type EventType string

const (
    EventLeaderChanged EventType = "leader_changed"
    EventElectionStart EventType = "election_start" // reserved, future
    EventElectionEnd   EventType = "election_end"   // reserved, future
    EventMemberJoin    EventType = "member_join"
    EventMemberLeave   EventType = "member_leave"
    EventMemberFailed  EventType = "member_failed"
)

// Event is an application-consumable event describing cluster state changes.
// Only relevant fields for an event type are populated.
type Event struct {
    Type     EventType
    At       time.Time
    Leader   *consensus.LeaderInfo
    Member   *membership.MemberInfo
    Term     uint64
    Details  map[string]string
}

// Subscribe returns a channel of events. The returned channel is buffered and
// closed automatically when ctx is done. Events may be dropped if the consumer
// is too slow (best-effort delivery) to avoid back-pressuring internals.
func (c *Cluster) Subscribe(ctx context.Context) <-chan Event {
    ch := make(chan Event, 64)
    c.eb.add(ch)
    go func() {
        <-ctx.Done()
        c.eb.remove(ch)
        close(ch)
    }()
    return ch
}

// internal event bus
type eventBus struct {
    mu   sync.Mutex
    subs map[chan Event]struct{}
}

func (e *eventBus) add(ch chan Event) {
    e.mu.Lock()
    if e.subs == nil { e.subs = make(map[chan Event]struct{}) }
    e.subs[ch] = struct{}{}
    e.mu.Unlock()
}

func (e *eventBus) remove(ch chan Event) {
    e.mu.Lock()
    if e.subs != nil { delete(e.subs, ch) }
    e.mu.Unlock()
}

func (e *eventBus) publish(ev Event) {
    e.mu.Lock()
    for ch := range e.subs {
        select {
        case ch <- ev:
        default:
            // drop if receiver is slow
        }
    }
    e.mu.Unlock()
}

