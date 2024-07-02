package kubernetes

import (
	"encoding/json"
	"log"

	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws/iam"
	aws_eks "github.com/pulumi/pulumi-aws/sdk/v6/go/aws/eks"
	eks "github.com/pulumi/pulumi-eks/sdk/go/eks"
	"github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes"
	k8s "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
	"github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/helm/v3"
	k8s_meta "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
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

	_, err = eks.NewNodeGroupV2(
		ctx,
		"fixed-node-group",
		&eks.NodeGroupV2Args{
			Cluster:         cluster,
			InstanceType:    pulumi.String("t2.medium"),
			DesiredCapacity: pulumi.Int(5),
			MinSize:         pulumi.Int(3),
			MaxSize:         pulumi.Int(5),
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
			ClusterName:      clusterName,
			AddonName:        pulumi.String("aws-ebs-csi-driver"),
			AddonVersion:     pulumi.String("v1.31.0-eksbuild.1"),
			ResolveConflicts: pulumi.String("OVERWRITE"),
		},
		pulumi.DependsOn([]pulumi.Resource{cluster}),
	)
	if err != nil {
		log.Fatal(err)
	}

	cniAddon, err := aws_eks.NewAddon(
		ctx,
		"aws-eks-addon-cni",
		&aws_eks.AddonArgs{
			ClusterName:      clusterName,
			AddonName:        pulumi.String("vpc-cni"),
			AddonVersion:     pulumi.String("v1.18.2-eksbuild.1"),
			ResolveConflicts: pulumi.String("OVERWRITE"),
		},
		pulumi.DependsOn([]pulumi.Resource{cluster}),
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

	_, err = helm.NewChart(
		ctx,
		"argocd",
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

	return &Cluster{
		eksCluster: cluster,
	}
}