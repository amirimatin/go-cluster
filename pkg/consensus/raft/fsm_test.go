package raftcons

import (
    "encoding/json"
    "testing"

    r "github.com/hashicorp/raft"

    m "github.com/amirimatin/go-cluster/pkg/membership"
    c "github.com/amirimatin/go-cluster/pkg/consensus"
    sm "github.com/amirimatin/go-cluster/pkg/state/membership"
)

func TestMembershipFSM_Apply_AddRemove(t *testing.T) {
    st := sm.New()
    fsm := newMembershipFSM(st)

    // Add node
    payload, _ := json.Marshal(m.MemberInfo{ID: "n1", Addr: "127.0.0.1:1"})
    cmd := c.Command{Op: "AddNode", Payload: payload}
    data, _ := json.Marshal(cmd)
    if v := fsm.Apply(&r.Log{Data: data}); v != nil {
        if err, ok := v.(error); ok && err != nil {
            t.Fatalf("apply add: %v", err)
        }
    }

    // Remove node
    rmPayload, _ := json.Marshal(struct{ID string `json:"id"`}{ID: "n1"})
    rm := c.Command{Op: "RemoveNode", Payload: rmPayload}
    rmdata, _ := json.Marshal(rm)
    if v := fsm.Apply(&r.Log{Data: rmdata}); v != nil {
        if err, ok := v.(error); ok && err != nil {
            t.Fatalf("apply remove: %v", err)
        }
    }
}

