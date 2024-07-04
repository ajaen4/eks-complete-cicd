package kubernetes

import (
	"log"
	"os"

	"github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes"
	"github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/apiextensions"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"gopkg.in/yaml.v2"

	k8s "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
	"github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/helm/v3"
	k8s_meta "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
)

type CICD struct {
	ctx          *pulumi.Context
	kubeProvider *kubernetes.Provider
	namespace    *k8s.Namespace
}

func NewCICD(ctx *pulumi.Context, kubeProvider *kubernetes.Provider) *CICD {
	cicd := &CICD{
		ctx:          ctx,
		kubeProvider: kubeProvider,
	}

	cicdNsName := pulumi.String("cicd")
	var err error
	cicd.namespace, err = k8s.NewNamespace(
		ctx,
		"cicd-namespace",
		&k8s.NamespaceArgs{
			Metadata: k8s_meta.ObjectMetaArgs{
				Name: cicdNsName,
			},
		},
		pulumi.Provider(cicd.kubeProvider),
	)
	if err != nil {
		log.Fatal(err)
	}

	argocdChart, err := helm.NewChart(
		ctx,
		"argo-cd",
		helm.ChartArgs{
			Namespace: cicdNsName,
			Chart:     pulumi.String("argo-cd"),
			Version:   pulumi.String("7.3.3"),
			FetchArgs: helm.FetchArgs{
				Repo: pulumi.String("https://argoproj.github.io/argo-helm"),
			},
		},
		pulumi.Provider(cicd.kubeProvider),
		pulumi.DependsOn([]pulumi.Resource{cicd.namespace}),
	)
	if err != nil {
		log.Fatal(err)
	}

	argocdApps, err := os.ReadFile("argo-cd-apps.yaml")
	if err != nil {
		log.Fatal(err)
	}

	argocdAppsFmt := map[string][]map[string]string{}
	err = yaml.Unmarshal(argocdApps, argocdAppsFmt)
	if err != nil {
		log.Fatal(err)
	}

	cicd.createArgoApps(argocdAppsFmt["applications"], argocdChart)

	return cicd
}

func (cicd *CICD) createArgoApps(argoApps []map[string]string, argocdChart *helm.Chart) {
	for _, app := range argoApps {
		_, err := apiextensions.NewCustomResource(
			cicd.ctx,
			"monitoring-app-cicd",
			&apiextensions.CustomResourceArgs{
				ApiVersion: pulumi.String("argoproj.io/v1alpha1"),
				Kind:       pulumi.String("Application"),
				Metadata: k8s_meta.ObjectMetaArgs{
					Name:      pulumi.String(app["name"]),
					Namespace: cicd.namespace.Metadata.Name(),
				},
				OtherFields: map[string]any{
					"spec": map[string]any{
						"project": "default",
						"source": map[string]any{
							"repoURL":        app["repoURL"],
							"path":           app["path"],
							"targetRevision": app["branch"],
						},
						"destination": map[string]any{
							"server": "https://kubernetes.default.svc",
						},
					},
				},
			},
			pulumi.Provider(cicd.kubeProvider),
			pulumi.DependsOnInputs(argocdChart.Ready),
		)

		if err != nil {
			log.Fatal(err)
		}
	}
}
