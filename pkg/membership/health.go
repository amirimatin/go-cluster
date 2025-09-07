package membership

// HealthReporter is an optional interface that a Membership implementation
// may provide to report a health score. Higher scores typically indicate
// degraded health according to the underlying implementation.
//
// Implementations that do not support health reporting can ignore this.
type HealthReporter interface {
    // HealthScore returns an integer health score. The exact semantics are
    // implementation-defined. A return value of -1 indicates the implementation
    // is not started or unavailable.
    HealthScore() int
}

