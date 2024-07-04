package main

import (
	"grafana-kubernetes/internal/kubernetes"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cluster := kubernetes.NewCluster(ctx)
		kubernetes.NewCICD(ctx, cluster.KubeProvider)
		return nil
	})
}
