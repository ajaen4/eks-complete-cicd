package kubernetes

import (
	"encoding/json"
	"log"
	"os"

	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws/iam"
	aws_eks "github.com/pulumi/pulumi-aws/sdk/v6/go/aws/eks"
	eks "github.com/pulumi/pulumi-eks/sdk/go/eks"
	"github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes"
	"github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/apiextensions"
	batchv1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/batch/v1"
	k8s "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
	"github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/helm/v3"
	k8s_meta "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	rbacv1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/rbac/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"gopkg.in/yaml.v2"
)

type Cluster struct {
	eksCluster *eks.Cluster
}

func NewCluster(ctx *pulumi.Context) *Cluster {
	managedPolicies := []string{
		"arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy",
		"arn:aws:iam::aws:policy/AmazonEKS_CNI_Policy",
		"arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly",
		"arn:aws:iam::aws:policy/service-role/AmazonEBSCSIDriverPolicy",
	}
	jsonPolicy, err := json.Marshal(map[string]any{
		"Version": "2012-10-17",
		"Statement": []map[string]any{
			{
				"Action": "sts:AssumeRole",
				"Effect": "Allow",
				"Principal": map[string]string{
					"Service": "ec2.amazonaws.com",
				},
			},
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	assumeRolePolicy := string(jsonPolicy)
	instanceRole, err := iam.NewRole(
		ctx,
		"instance-role",
		&iam.RoleArgs{
			AssumeRolePolicy:  pulumi.String(assumeRolePolicy),
			ManagedPolicyArns: pulumi.ToStringArray(managedPolicies),
		},
	)
	if err != nil {
		log.Fatal(err)
	}
	instanceProfile, err := iam.NewInstanceProfile(
		ctx,
		"instance-profile",
		&iam.InstanceProfileArgs{
			Role: instanceRole.Name,
		},
	)
	if err != nil {
		log.Fatal(err)
	}

	clusterName := pulumi.Sprintf("eks-cluster-%s", ctx.Stack())
	cluster, err := eks.NewCluster(
		ctx,
		"eks-cluster",
		&eks.ClusterArgs{
			Name:                 clusterName,
			DesiredCapacity:      pulumi.Int(5),
			MinSize:              pulumi.Int(3),
			MaxSize:              pulumi.Int(5),
			SkipDefaultNodeGroup: pulumi.BoolRef(true),
			InstanceRole:         instanceRole,
			EnabledClusterLogTypes: pulumi.StringArray{
				pulumi.String("api"),
				pulumi.String("audit"),
				pulumi.String("authenticator"),
			},
		},
	)
	if err != nil {
		log.Fatal(err)
	}

	nodeGroup, err := eks.NewNodeGroupV2(
		ctx,
		"fixed-node-group",
		&eks.NodeGroupV2Args{
			Cluster:         cluster,
			InstanceType:    pulumi.String("t4g.small"),
			AmiType:         pulumi.String("amazon-linux-2-arm64"),
			DesiredCapacity: pulumi.Int(5),
			MinSize:         pulumi.Int(5),
			MaxSize:         pulumi.Int(7),
			Labels: map[string]string{
				"ondemand": "true",
			},
			InstanceProfile: instanceProfile,
		})
	if err != nil {
		log.Fatal(err)
	}

	csiAddon, err := aws_eks.NewAddon(
		ctx,
		"aws-eks-addon-csi",
		&aws_eks.AddonArgs{
			ClusterName:              clusterName,
			AddonName:                pulumi.String("aws-ebs-csi-driver"),
			AddonVersion:             pulumi.String("v1.31.0-eksbuild.1"),
			ResolveConflictsOnCreate: pulumi.String("OVERWRITE"),
			ResolveConflictsOnUpdate: pulumi.String("OVERWRITE"),
		},
		pulumi.DependsOn([]pulumi.Resource{nodeGroup}),
	)
	if err != nil {
		log.Fatal(err)
	}

	cniAddon, err := aws_eks.NewAddon(
		ctx,
		"aws-eks-addon-cni",
		&aws_eks.AddonArgs{
			ClusterName:              clusterName,
			AddonName:                pulumi.String("vpc-cni"),
			AddonVersion:             pulumi.String("v1.18.2-eksbuild.1"),
			ResolveConflictsOnCreate: pulumi.String("OVERWRITE"),
			ResolveConflictsOnUpdate: pulumi.String("OVERWRITE"),
		},
		pulumi.DependsOn([]pulumi.Resource{nodeGroup}),
	)
	if err != nil {
		log.Fatal(err)
	}

	kubeProvider, err := kubernetes.NewProvider(
		ctx,
		"k8s-provider",
		&kubernetes.ProviderArgs{
			Kubeconfig: cluster.KubeconfigJson,
		},
		pulumi.DependsOn([]pulumi.Resource{csiAddon, cniAddon}),
	)
	if err != nil {
		log.Fatal(err)
	}

	cicdNsName := pulumi.String("cicd")
	_, err = k8s.NewNamespace(
		ctx,
		"cicd-namespace",
		&k8s.NamespaceArgs{
			Metadata: k8s_meta.ObjectMetaArgs{
				Name: cicdNsName,
			},
		},
		pulumi.Provider(kubeProvider),
	)
	if err != nil {
		log.Fatal(err)
	}

	clusterRole, err := rbacv1.NewClusterRole(
		ctx,
		"crd-patch-clusterrole",
		&rbacv1.ClusterRoleArgs{
			Metadata: &k8s_meta.ObjectMetaArgs{
				Name:      pulumi.String("crd-patch-clusterrole"),
				Namespace: cicdNsName,
			},
			Rules: rbacv1.PolicyRuleArray{
				&rbacv1.PolicyRuleArgs{
					ApiGroups: pulumi.StringArray{
						pulumi.String("apiextensions.k8s.io"),
					},
					Resources: pulumi.StringArray{
						pulumi.String("customresourcedefinitions"),
					},
					Verbs: pulumi.StringArray{
						pulumi.String("get"),
						pulumi.String("list"),
						pulumi.String("watch"),
						pulumi.String("patch"),
						pulumi.String("update"),
						pulumi.String("create"),
					},
				},
			},
		},
		pulumi.Provider(kubeProvider),
	)
	if err != nil {
		log.Fatal(err)
	}

	roleBinding, err := rbacv1.NewClusterRoleBinding(
		ctx,
		"crd-patch-clusterrolebinding",
		&rbacv1.ClusterRoleBindingArgs{
			Metadata: &k8s_meta.ObjectMetaArgs{
				Name:      pulumi.String("crd-patch-clusterrole-binding"),
				Namespace: cicdNsName,
			},
			Subjects: rbacv1.SubjectArray{
				&rbacv1.SubjectArgs{
					Kind:      pulumi.String("ServiceAccount"),
					Name:      pulumi.String("default"),
					Namespace: cicdNsName,
				},
			},
			RoleRef: &rbacv1.RoleRefArgs{
				Kind:     pulumi.String("ClusterRole"),
				Name:     pulumi.String("crd-patch-clusterrole"),
				ApiGroup: pulumi.String("rbac.authorization.k8s.io"),
			},
		},
		pulumi.Provider(kubeProvider),
		pulumi.DependsOn([]pulumi.Resource{clusterRole}),
	)
	if err != nil {
		log.Fatal(err)
	}

	batchv1.NewJob(
		ctx,
		"cluster-init-job",
		&batchv1.JobArgs{
			ApiVersion: pulumi.String("batch/v1"),
			Kind:       pulumi.String("Job"),
			Metadata: k8s_meta.ObjectMetaArgs{
				Name:      pulumi.String("cluster-init-job"),
				Namespace: cicdNsName,
			},
			Spec: batchv1.JobSpecArgs{
				Template: k8s.PodTemplateSpecArgs{
					Spec: k8s.PodSpecArgs{
						Containers: k8s.ContainerArray{
							k8s.ContainerArgs{
								Name:    pulumi.String("cluster-init-job"),
								Image:   pulumi.String("bitnami/kubectl:latest"),
								Command: pulumi.StringArray{pulumi.String("/bin/sh"), pulumi.String("-c")},
								Args: pulumi.StringArray{
									pulumi.String(`kubectl apply --server-side -f https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/v0.75.0/example/prometheus-operator-crd/monitoring.coreos.com_alertmanagerconfigs.yaml && \
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
						RestartPolicy: pulumi.String("OnFailure"),
					},
				},
			},
		},
		pulumi.Provider(kubeProvider),
		pulumi.DependsOn([]pulumi.Resource{roleBinding}),
	)

	_, err = helm.NewChart(
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
		pulumi.Provider(kubeProvider),
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

	for _, app := range argocdAppsFmt["applications"] {
		_, err = apiextensions.NewCustomResource(
			ctx,
			"monitoring-app-cicd",
			&apiextensions.CustomResourceArgs{
				ApiVersion: pulumi.String("argoproj.io/v1alpha1"),
				Kind:       pulumi.String("Application"),
				Metadata: k8s_meta.ObjectMetaArgs{
					Name:      pulumi.String(app["name"]),
					Namespace: cicdNsName,
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
			pulumi.Provider(kubeProvider),
		)

		if err != nil {
			log.Fatal(err)
		}
	}

	return &Cluster{
		eksCluster: cluster,
	}
}
