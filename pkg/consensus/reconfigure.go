package consensus

import "time"

// Reconfigurer optionally allows dynamic membership reconfiguration
// (adding/removing servers) in the underlying consensus engine.
// Implementations that support it should satisfy this interface.
type Reconfigurer interface {
    AddVoter(id, addr string, timeout time.Duration) error
    RemoveServer(id string, timeout time.Duration) error
}

