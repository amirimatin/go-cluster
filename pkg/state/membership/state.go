package membership

import (
    "encoding/json"
    "fmt"
    "sort"
    "sync"

    m "github.com/amirimatin/go-cluster/pkg/membership"
    base "github.com/amirimatin/go-cluster/pkg/state"
)

// State is a simple in-memory FSM for cluster membership.
type State struct {
    mu      sync.RWMutex
    members map[string]m.MemberInfo
}

func New() *State { return &State{members: make(map[string]m.MemberInfo)} }

func (s *State) ApplyAddNode(n m.MemberInfo) error {
    if n.ID == "" { return fmt.Errorf("state: empty node id") }
    s.mu.Lock(); defer s.mu.Unlock()
    s.members[n.ID] = n
    return nil
}

func (s *State) ApplyRemoveNode(nodeID string) error {
    if nodeID == "" { return fmt.Errorf("state: empty node id") }
    s.mu.Lock(); defer s.mu.Unlock()
    delete(s.members, nodeID)
    return nil
}

// Snapshot encodes state as a stable JSON for ease of debugging/migration.
func (s *State) Snapshot() ([]byte, error) {
    s.mu.RLock(); defer s.mu.RUnlock()
    arr := make([]m.MemberInfo, 0, len(s.members))
    for _, v := range s.members { arr = append(arr, v) }
    sort.Slice(arr, func(i, j int) bool { return arr[i].ID < arr[j].ID })
    return json.Marshal(struct{
        Version int               `json:"version"`
        Members []m.MemberInfo    `json:"members"`
    }{Version: 1, Members: arr})
}

func (s *State) Restore(buf []byte) error {
    var snapshot struct{
        Version int             `json:"version"`
        Members []m.MemberInfo  `json:"members"`
    }
    if err := json.Unmarshal(buf, &snapshot); err != nil {
        return err
    }
    // For now we only support Version 1.
    s.mu.Lock(); defer s.mu.Unlock()
    s.members = make(map[string]m.MemberInfo, len(snapshot.Members))
    for _, v := range snapshot.Members {
        if v.ID == "" { continue }
        s.members[v.ID] = v
    }
    return nil
}

// Ensure interface satisfaction at compile-time.
var _ base.MembershipState = (*State)(nil)

