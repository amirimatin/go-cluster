package main

import (
    "log"

    "github.com/spf13/cobra"

    clustercli "github.com/amirimatin/go-cluster/pkg/cli"
)

func main() {
    if err := newRoot().Execute(); err != nil {
        log.Fatal(err)
    }
}

func newRoot() *cobra.Command {
    root := &cobra.Command{
        Use:           "clusterctl",
        Short:         "go-cluster management CLI",
        SilenceUsage:  true,
        SilenceErrors: true,
    }
    // Attach all cluster commands from pkg/cli for reuse in services
    clustercli.AddAll(root)
    return root
}
