package main

import (
	"grafana-kubernetes/internal/kubernetes"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		kubernetes.NewCluster(ctx)
		return nil
	})
}
