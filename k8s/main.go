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

	roleName := jsii.String("job-init-cluster-role")
	clusterRole := k8s.NewKubeClusterRole(
		chart,
		roleName,
		&k8s.KubeClusterRoleProps{
			Metadata: &k8s.ObjectMeta{
				Name: roleName,
			},
			Rules: &[]*k8s.PolicyRule{
				{
					ApiGroups: &[]*string{
						jsii.String("apiextensions.k8s.io"),
					},
					Resources: &[]*string{
						jsii.String("customresourcedefinitions"),
					},
					Verbs: &[]*string{
						jsii.String("get"),
						jsii.String("list"),
						jsii.String("watch"),
						jsii.String("patch"),
						jsii.String("update"),
						jsii.String("create"),
					},
				},
			},
		},
	)
	clusterRole.AddDependency(monitoringNs)

	bindingName := jsii.String("job-init-role-binding")
	roleBinding := k8s.NewKubeClusterRoleBinding(
		chart,
		bindingName,
		&k8s.KubeClusterRoleBindingProps{
			Metadata: &k8s.ObjectMeta{
				Name:      bindingName,
				Namespace: monitoringNs.Name(),
			},
			Subjects: &[]*k8s.Subject{
				{
					Kind:      jsii.String("ServiceAccount"),
					Name:      jsii.String("default"),
					Namespace: monitoringNs.Name(),
				},
			},
			RoleRef: &k8s.RoleRef{
				Kind:     jsii.String("ClusterRole"),
				Name:     clusterRole.Name(),
				ApiGroup: jsii.String("rbac.authorization.k8s.io"),
			},
		},
	)
	roleBinding.AddDependency(clusterRole)

	job := k8s.NewKubeJob(
		chart,
		jsii.String("cluster-init-job"),
		&k8s.KubeJobProps{
			Metadata: &k8s.ObjectMeta{
				Name:      jsii.String("cluster-init-job"),
				Namespace: monitoringNs.Name(),
			},
			Spec: &k8s.JobSpec{
				Template: &k8s.PodTemplateSpec{
					Spec: &k8s.PodSpec{
						Containers: &[]*k8s.Container{
							{
								Name:    jsii.String("cluster-init-job"),
								Image:   jsii.String("bitnami/kubectl:latest"),
								Command: &[]*string{jsii.String("/bin/sh"), jsii.String("-c")},
								Args: &[]*string{
									jsii.String(`kubectl apply --server-side -f https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/v0.75.0/example/prometheus-operator-crd/monitoring.coreos.com_alertmanagerconfigs.yaml && \
									kubectl apply --server-side -f https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/v0.75.0/example/prometheus-operator-crd/monitoring.coreos.com_alertmanagers.yaml && \
									kubectl apply --server-side -f https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/v0.75.0/example/prometheus-operator-crd/monitoring.coreos.com_podmonitors.yaml && \
									kubectl apply --server-side -f https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/v0.75.0/example/prometheus-operator-crd/monitoring.coreos.com_probes.yaml && \
									kubectl apply --server-side -f https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/v0.75.0/example/prometheus-operator-crd/monitoring.coreos.com_prometheusagents.yaml && \
									kubectl apply --server-side -f https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/v0.75.0/example/prometheus-operator-crd/monitoring.coreos.com_prometheuses.yaml && \
									kubectl apply --server-side -f https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/v0.75.0/example/prometheus-operator-crd/monitoring.coreos.com_prometheusrules.yaml && \
									kubectl apply --server-side -f https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/v0.75.0/example/prometheus-operator-crd/monitoring.coreos.com_scrapeconfigs.yaml && \
									kubectl apply --server-side -f https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/v0.75.0/example/prometheus-operator-crd/monitoring.coreos.com_servicemonitors.yaml && \
									kubectl apply --server-side -f https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/v0.75.0/example/prometheus-operator-crd/monitoring.coreos.com_thanosrulers.yaml`),
								},
							},
						},
						RestartPolicy: jsii.String("OnFailure"),
					},
				},
			},
		},
	)
	job.Node().AddDependency(roleBinding)

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
	).Node().AddDependency(monitoringNs, job)

	return chart
}

func main() {
	app := cdk8s.NewApp(nil)
	NewMonitoring(app, "monitoring", nil)
	app.Synth()
}
