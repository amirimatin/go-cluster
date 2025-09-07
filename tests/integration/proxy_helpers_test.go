//go:build integration

package integration

import (
    "context"
    "encoding/json"
    httpjson "github.com/amirimatin/go-cluster/pkg/transport/httpjson"
)

type status2 struct {
    Healthy    bool   `json:"Healthy"`
    Term       uint64 `json:"Term"`
    LeaderID   string `json:"LeaderID"`
    LeaderAddr string `json:"LeaderAddr"`
}

func fetchStatus2h(ctx context.Context, cli *httpjson.Client, addr string) (status2, error) {
    var s status2
    b, err := cli.GetStatus(ctx, addr)
    if err != nil { return s, err }
    if err := json.Unmarshal(b, &s); err != nil { return s, err }
    return s, nil
}
