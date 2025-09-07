# Known Issues and Improvement Tasks

This document collects potential bugs, risks, and improvement tasks discovered during review. No logic was changed as part of this pass.

## Potential Bugs / Clarifications

- Tutorial snippet uses `bootstrap.LeaderInfo` in callback examples, while the actual type lives in `pkg/consensus` (`consensus.LeaderInfo`). The example should import the correct type or refer to it generically. (docs)
- Backpressure window calculation in `Publish`: if `minAck` equals zero for a long time, publishing may block. This is expected during cold start but should be documented. Consider an initial warmup window. (behavior-by-design but worth documenting)
- Replication topic sanitization for disk buffer uses a simple replacer. If different topics sanitize to the same string, files could collide. Consider namespacing by hashing the original topic while keeping a readable prefix. (reliability)
- Election detection relies on polling `Leader()` when the consensus does not emit explicit election lifecycle events. This heuristic could produce spurious `ElectionStart` on transient leader observation gaps. Consider optional debounce. (UX)
- HTTP JSON management endpoints do not authenticate/authorize. Production guidance should mandate network policy and TLS client auth (mTLS). (security/docs)
- Memberlist events are dropped when the channel is full. This is intentional to avoid blocking, but adding a metric for dropped events could help visibility. (observability)

## Code Quality / SOLID Improvements (Non-breaking)

- Add per-package GoDoc package comments (package-level) so `godoc` renders module overview.
- Split `pkg/transport/grpc/server.go` into smaller files (management vs replication) to improve SRP and readability.
- Introduce interfaces for replication broadcasting and ack state to allow unit testing without gRPC server concrete type.
- Add logging/context annotations for `Publish` retries and backpressure waits to aid ops troubleshooting.

## Tests / CI

- Add a smoke test for `Publish` with disk buffer enabled to assert on file creation/cleanup lifecycle.
- Add a test to verify callbacks `OnElectionStart/End` fire in expected sequences during leader stop/re-election.

## Docs

- Extend TLS/mTLS how-to with OpenSSL commands and folder layout best practices.
- Add a section in Monitoring for common SLOs, e.g., max acceptable lag and alert templates.

