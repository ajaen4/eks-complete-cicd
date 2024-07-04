package main

import (
	"log"
	"os/exec"

	"k8s/imports/k8s"

	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
	"github.com/cdk8s-team/cdk8s-core-go/cdk8s/v2"
)

type MyChartProps struct {
	cdk8s.ChartProps
}

func NewMonitoring(scope constructs.Construct, id string, props *MyChartProps) cdk8s.Chart {
	var cprops cdk8s.ChartProps
	if props != nil {
		cprops = props.ChartProps
	}

	chart := cdk8s.NewChart(scope, jsii.String(id), &cprops)

	nsName := "monitoring"
	monitoringNs := k8s.NewKubeNamespace(
		chart,
		&nsName,
		&k8s.KubeNamespaceProps{
			Metadata: &k8s.ObjectMeta{
				Name: &nsName,
			},
		},
	)

	repos := map[string]string{
		"bitnami":              "https://charts.bitnami.com/bitnami",
		"prometheus-community": "https://prometheus-community.github.io/helm-charts",
	}
	var err error
	for name, url := range repos {
		err = exec.Command("helm", "repo", "add", name, url).Run()
		if err != nil {
			log.Fatal(err)
		}
	}

	err = exec.Command("helm", "repo", "update").Run()
	if err != nil {
		log.Fatal(err)
	}

	releaseName := jsii.String("metrics-server")
	cdk8s.NewHelm(
		chart,
		releaseName,
		&cdk8s.HelmProps{
			ReleaseName: releaseName,
			Namespace:   monitoringNs.Name(),
			Chart:       jsii.String("bitnami/metrics-server"),
			Version:     jsii.String("7.2.6"),
		},
	).Node().AddDependency(monitoringNs)

	releaseName = jsii.String("monitoring-stack")
	cdk8s.NewHelm(
		chart,
		releaseName,
		&cdk8s.HelmProps{
			ReleaseName: releaseName,
			Namespace:   monitoringNs.Name(),
			Chart:       jsii.String("prometheus-community/kube-prometheus-stack"),
			Version:     jsii.String("61.1.1"),
		},
	).Node().AddDependency(monitoringNs)

	return chart
}

func main() {
	app := cdk8s.NewApp(nil)
	NewMonitoring(app, "monitoring", nil)
	app.Synth()
}
