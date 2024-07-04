package main

import (
	"k8s/internal/apps"

	"github.com/cdk8s-team/cdk8s-core-go/cdk8s/v2"
)

func main() {
	app := cdk8s.NewApp(nil)
	apps.NewMonitoring(app, "monitoring", nil)
	app.Synth()
}
