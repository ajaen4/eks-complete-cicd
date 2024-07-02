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

	releaseName := "metrics-server"
	chartName := "bitnami/metrics-server"
	version := "7.2.6"
	cdk8s.NewHelm(
		chart,
		&chartName,
		&cdk8s.HelmProps{
			ReleaseName: &releaseName,
			Namespace:   &nsName,
			Chart:       &chartName,
			Version:     &version,
		},
	).Node().AddDependency(monitoringNs)

	chartName = "preSyncJob"
	preSyncJob := k8s.NewKubeJob(
		chart,
		&chartName,
		&k8s.KubeJobProps{
			Metadata: &k8s.ObjectMeta{
				Name:      &chartName,
				Namespace: &nsName,
				Annotations: &map[string]*string{
					"argocd.argoproj.io/hook": jsii.String("PreSync"),
				},
			},
			Spec: &k8s.JobSpec{
				Template: &k8s.PodTemplateSpec{
					Spec: &k8s.PodSpec{
						Containers: &[]*k8s.Container{
							{
								Name:  &chartName,
								Image: jsii.String("bitnami/kubectl:latest"),
								Command: &[]*string{
									jsii.String("/bin/sh"),
									jsii.String("-c"),
								},
								Args: &[]*string{
									jsii.String("kubectl apply --server-side -f https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/v0.75.0/example/prometheus-operator-crd/monitoring.coreos.com_alertmanagerconfigs.yaml"),
									jsii.String("kubectl apply --server-side -f https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/v0.75.0/example/prometheus-operator-crd/monitoring.coreos.com_alertmanagers.yaml"),
									jsii.String("kubectl apply --server-side -f https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/v0.75.0/example/prometheus-operator-crd/monitoring.coreos.com_podmonitors.yaml"),
									jsii.String("kubectl apply --server-side -f https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/v0.75.0/example/prometheus-operator-crd/monitoring.coreos.com_probes.yaml"),
									jsii.String("kubectl apply --server-side -f https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/v0.75.0/example/prometheus-operator-crd/monitoring.coreos.com_prometheusagents.yaml"),
									jsii.String("kubectl apply --server-side -f https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/v0.75.0/example/prometheus-operator-crd/monitoring.coreos.com_prometheuses.yaml"),
									jsii.String("kubectl apply --server-side -f https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/v0.75.0/example/prometheus-operator-crd/monitoring.coreos.com_prometheusrules.yaml"),
									jsii.String("kubectl apply --server-side -f https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/v0.75.0/example/prometheus-operator-crd/monitoring.coreos.com_scrapeconfigs.yaml"),
									jsii.String("kubectl apply --server-side -f https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/v0.75.0/example/prometheus-operator-crd/monitoring.coreos.com_servicemonitors.yaml"),
									jsii.String("kubectl apply --server-side -f https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/v0.75.0/example/prometheus-operator-crd/monitoring.coreos.com_thanosrulers.yaml"),
								},
							},
						},
						RestartPolicy: jsii.String("OnFailure"),
					},
				},
			},
		},
	)

	releaseName = "monitoring-stack"
	chartName = "prometheus-community/kube-prometheus-stack"
	version = "61.1.1"
	cdk8s.NewHelm(
		chart,
		&chartName,
		&cdk8s.HelmProps{
			ReleaseName: &releaseName,
			Namespace:   &nsName,
			Chart:       &chartName,
			Version:     &version,
		},
	).Node().AddDependency(monitoringNs, preSyncJob)

	return chart
}

func main() {
	app := cdk8s.NewApp(nil)
	NewMonitoring(app, "monitoring", nil)
	app.Synth()
}
