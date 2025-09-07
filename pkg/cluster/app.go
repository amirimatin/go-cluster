package cluster

import "context"

// AppHandlers defines the application-level callbacks that a service can
// implement and inject into the cluster to handle reads/writes/sync without
// creating dependencies to storage or external systems inside this package.
// All payloads are opaque to the cluster and passed through as []byte.
type AppHandlers interface {
    HandleWrite(ctx context.Context, op string, req []byte) (resp []byte, err error)
    HandleRead(ctx context.Context, op string, req []byte) (resp []byte, err error)
    HandleSync(ctx context.Context, topic string, data []byte) error
}

