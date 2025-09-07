package raftcons

import (
    "encoding/json"
    "io"
    "time"

    "github.com/hashicorp/raft"

    m "github.com/amirimatin/go-cluster/pkg/membership"
    sm "github.com/amirimatin/go-cluster/pkg/state/membership"
    base "github.com/amirimatin/go-cluster/pkg/state"
    c "github.com/amirimatin/go-cluster/pkg/consensus"
)

// membershipFSM bridges Raft Apply/Snapshot to our MembershipState.
type membershipFSM struct {
    ms base.MembershipState
}

func newMembershipFSM(ms base.MembershipState) *membershipFSM { return &membershipFSM{ms: ms} }

func (f *membershipFSM) Apply(l *raft.Log) interface{} {
    var cmd c.Command
    if err := json.Unmarshal(l.Data, &cmd); err != nil {
        return err
    }
    switch cmd.Op {
    case "AddNode":
        var mi m.MemberInfo
        if err := json.Unmarshal(cmd.Payload, &mi); err != nil { return err }
        return f.ms.ApplyAddNode(mi)
    case "RemoveNode":
        var req struct{ ID string `json:"id"` }
        if err := json.Unmarshal(cmd.Payload, &req); err != nil { return err }
        return f.ms.ApplyRemoveNode(req.ID)
    default:
        return nil
    }
}

func (f *membershipFSM) Snapshot() (raft.FSMSnapshot, error) {
    blob, err := f.ms.Snapshot()
    if err != nil { return nil, err }
    return &snapshot{blob: blob, at: time.Now()}, nil
}

func (f *membershipFSM) Restore(rc io.ReadCloser) error {
    defer rc.Close()
    data, err := io.ReadAll(rc)
    if err != nil { return err }
    return f.ms.Restore(data)
}

type snapshot struct {
    blob []byte
    at   time.Time
}

func (s *snapshot) Persist(sink raft.SnapshotSink) error {
    if _, err := sink.Write(s.blob); err != nil { _ = sink.Cancel(); return err }
    return sink.Close()
}

func (s *snapshot) Release() {}

// Ensure compile-time interface compliance.
var _ raft.FSM = (*membershipFSM)(nil)
var _ base.MembershipState = (*sm.State)(nil)

