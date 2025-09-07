package membership

import (
    "context"
    "time"
)

// MemberInfo describes a cluster member as observed by the membership layer
// (e.g., memberlist). Meta can carry auxiliary data such as management address.
type MemberInfo struct {
    ID   string
    Addr string
    Meta map[string]string
}

type EventType string

const (
    // EventJoin indicates a member joined or became visible.
    EventJoin   EventType = "join"
    // EventLeave indicates a member left the cluster.
    EventLeave  EventType = "leave"
    // EventFailed indicates membership marked the node as failed/unreachable.
    EventFailed EventType = "failed"
)

// Event is the translated membership change notification.
type Event struct {
    Type   EventType
    Member MemberInfo
    At     time.Time
}

// Membership is the abstraction over the underlying gossip/failure-detection
// layer. It is responsible for peer discovery, join/leave and event delivery.
type Membership interface {
    Start(ctx context.Context) error
    Join(seeds []string) error
    Local() MemberInfo
    Members() []MemberInfo
    Events() <-chan Event
    Leave() error
    Stop() error
}
