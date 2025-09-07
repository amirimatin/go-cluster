# Changelog

All notable changes to this project will be documented in this file.

## v0.1.0

- Membership (memberlist) with join/leave events and health score
- Raft consensus for membership FSM (add/remove) with leader election
- Management transports: HTTP/JSON and gRPC (JSON codec)
- CLI `clusterctl` with `run/status/join/leave`
- Discovery backends: static, file+ENV, DNS (SRV/A)
- Observability: Prometheus metrics, basic logging, tracing hooks
- Management mTLS (server+client), optional, with hot-reload of certs
- gRPC connection manager (cache + idle eviction)
- Integration tests: 3-node join/status, leader change, leave/converge, TLS, AppWrite forward, Publish fanout
- Replication (gRPC stream): Subscribe with keepalive + ConnManager reuse; Seq/Ack support, per-node ack counters, lag gauges, window/backpressure, and retry buffer
- Docker Compose example (3-node)

Refer to `DEVELOPMENT_PLAN.md` and `MILESTONES.md` for roadmap and status.
