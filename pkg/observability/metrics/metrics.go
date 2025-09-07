package metrics

import (
    "sync"

    "github.com/prometheus/client_golang/prometheus"
)

var (
    once sync.Once

    ClusterMembers = prometheus.NewGauge(prometheus.GaugeOpts{
        Namespace: "go_cluster",
        Name:      "members_total",
        Help:      "Current number of known cluster members",
    })

    IsLeader = prometheus.NewGauge(prometheus.GaugeOpts{
        Namespace: "go_cluster",
        Name:      "is_leader",
        Help:      "1 if this node is the leader, else 0",
    })

    LeaderChanges = prometheus.NewCounter(prometheus.CounterOpts{
        Namespace: "go_cluster",
        Name:      "leader_changes_total",
        Help:      "Total number of observed leader change events",
    })

    JoinRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
        Namespace: "go_cluster",
        Name:      "join_requests_total",
        Help:      "Total join requests handled by this node",
    }, []string{"result"})

    GRPCConnDials = prometheus.NewCounter(prometheus.CounterOpts{
        Namespace: "go_cluster",
        Subsystem: "grpc_conn",
        Name:      "dials_total",
        Help:      "Total number of new gRPC connections dialed",
    })
    GRPCConnReuse = prometheus.NewCounter(prometheus.CounterOpts{
        Namespace: "go_cluster",
        Subsystem: "grpc_conn",
        Name:      "reuse_total",
        Help:      "Total number of gRPC connection reuses from cache",
    })
    GRPCConnEvictions = prometheus.NewCounter(prometheus.CounterOpts{
        Namespace: "go_cluster",
        Subsystem: "grpc_conn",
        Name:      "evictions_total",
        Help:      "Total number of cached gRPC connections evicted",
    })
    GRPCConnActive = prometheus.NewGauge(prometheus.GaugeOpts{
        Namespace: "go_cluster",
        Subsystem: "grpc_conn",
        Name:      "active",
        Help:      "Number of active cached gRPC connections",
    })

    // Replication metrics
    ReplicationPublishedTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
        Namespace: "go_cluster",
        Subsystem: "repl",
        Name:      "published_total",
        Help:      "Total number of replication publishes (leader side)",
    }, []string{"topic"})
    ReplicationBroadcastTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
        Namespace: "go_cluster",
        Subsystem: "repl",
        Name:      "broadcast_total",
        Help:      "Total number of replication messages broadcast to subscribers",
    }, []string{"topic"})
    ReplicationAckTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
        Namespace: "go_cluster",
        Subsystem: "repl",
        Name:      "acks_total",
        Help:      "Total number of replication acknowledgements received",
    }, []string{"topic"})
    ReplicationAckPerNodeTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
        Namespace: "go_cluster",
        Subsystem: "repl",
        Name:      "acks_per_node_total",
        Help:      "Total number of replication acknowledgements per node",
    }, []string{"topic","node"})
    ReplicationAckSeqPerNode = prometheus.NewGaugeVec(prometheus.GaugeOpts{
        Namespace: "go_cluster",
        Subsystem: "repl",
        Name:      "ack_seq_per_node",
        Help:      "Last acknowledged sequence per topic and node",
    }, []string{"topic","node"})
    ReplicationLagPerNode = prometheus.NewGaugeVec(prometheus.GaugeOpts{
        Namespace: "go_cluster",
        Subsystem: "repl",
        Name:      "lag_per_node",
        Help:      "Replication lag (seq - node_ack_seq) per topic and node",
    }, []string{"topic","node"})
    ReplicationSeq = prometheus.NewGaugeVec(prometheus.GaugeOpts{
        Namespace: "go_cluster",
        Subsystem: "repl",
        Name:      "seq",
        Help:      "Current published sequence per topic (leader side)",
    }, []string{"topic"})
    ReplicationAckSeq = prometheus.NewGaugeVec(prometheus.GaugeOpts{
        Namespace: "go_cluster",
        Subsystem: "repl",
        Name:      "ack_seq",
        Help:      "Last acknowledged sequence per topic (leader side)",
    }, []string{"topic"})
    ReplicationLag = prometheus.NewGaugeVec(prometheus.GaugeOpts{
        Namespace: "go_cluster",
        Subsystem: "repl",
        Name:      "lag",
        Help:      "Replication lag (seq - ack_seq) per topic",
    }, []string{"topic"})
    ReplicationSubs = prometheus.NewGauge(prometheus.GaugeOpts{
        Namespace: "go_cluster",
        Subsystem: "repl",
        Name:      "subs",
        Help:      "Number of active replication subscribers",
    })
)

// Register registers metrics into the default Prometheus registry (idempotent).
func Register() {
    once.Do(func() {
        prometheus.MustRegister(ClusterMembers)
        prometheus.MustRegister(IsLeader)
        prometheus.MustRegister(LeaderChanges)
        prometheus.MustRegister(JoinRequests)
        prometheus.MustRegister(GRPCConnDials)
        prometheus.MustRegister(GRPCConnReuse)
        prometheus.MustRegister(GRPCConnEvictions)
        prometheus.MustRegister(GRPCConnActive)
        // replication
        prometheus.MustRegister(ReplicationPublishedTotal)
        prometheus.MustRegister(ReplicationBroadcastTotal)
        prometheus.MustRegister(ReplicationAckTotal)
        prometheus.MustRegister(ReplicationAckPerNodeTotal)
        prometheus.MustRegister(ReplicationAckSeqPerNode)
        prometheus.MustRegister(ReplicationLagPerNode)
        prometheus.MustRegister(ReplicationSeq)
        prometheus.MustRegister(ReplicationAckSeq)
        prometheus.MustRegister(ReplicationLag)
        prometheus.MustRegister(ReplicationSubs)
    })
}
