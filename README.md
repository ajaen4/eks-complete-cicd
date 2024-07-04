# EKS Complete CI/CD

## Introduction

This project deploys a completely automatized EKS cluster leveraging Pulumi, Argo CD and CDK8s. The aim is to be able to automatically deploy a Kubernetes cluster with a single Pulumi deployment and set up the CI/CD for any Kubernetes application through Argo CD. We have used a CDK8s stack located in this same repo as an example of a Kubernetes app deployment.

So in other words, we want all configuration and deployments to be automatic, 0 manual configuration. We are also using tools that provide us with the ability to use general purpose programing languages, which provide more flexibility and integration that YAML or Domain Specific Languages.

## Architecture

![Alt text](imgs/eks-diagram.png?raw=true "EKS Diagram")

## Tech Stack

- Go
- AWS
- Kubernetes + kubectl
- Pulumi: IaC tool to be able to define infrastructure with a general purpose programming language instead of a domain specific one.
- CDK8s: To be able to define Kubernetes manifests with a general purpose programming language instead of YAML.

## Requirements

- You must have Go installed.
- You must own an AWS account.
- You must have Kubectl and Helm installed locally.

## Infrastructure deployed

This code will deploy the following infraestructure inside AWS:
- 1 EKS Cluster
    - Node Group with 5 t4g.medium instances. This can be changed easily in the code.
    - CSI Addon
    - CNI Addon
    - 1 cluster initialization job
- 1 Node Group
- Kubernetes applications:
    - Argo CD (Deployed with Pulumi)
    - Kubernetes Metrics Server (Deployed with ArgoCD)
    - Prometheus (Deployed with ArgoCD)
    - Grafana (Deployed with ArgoCD)

## Workflow

As a first step the Pulumi application is deployed, setting up an EKS cluster on AWS with an Argo CD application. The Pulumi application also contains a Kubernetes job to initialize the cluster, in this example it is used to install some CRDs needed for a Prometheus app.

The file "argo-cd-apps.yaml" defines in a simple manner the Kubernetes stacks that need to be deployed. In this example we have defined a CDK8s stack defined in this same repo in the "k8s" folder. It contains the necessary components to monitor a Kubernetes cluster: a metrics server to serve the cluster's metrics, a Prometheus server to store these and a Grafana server to display them.

In this file you can define any app you may like, you just need to add the config to the array. It is important to note that it doesn't need to be a CDK8s application, the only requirement is that all Kubernetes manifests are stored in a folder where our Argo CD server can find them.

## Installation

Follow the instructions [here](https://www.pulumi.com/docs/clouds/aws/get-started/) to get started with Pulumi.

Follow the instructions [here](https://cdk8s.io/docs/latest/get-started/) to get started with CDK8s.

## Step by step deployment

### 1. Configure Kubernetes apps

As explained in the Workflow section, we can define all our Kubernetes stacks in the file "argo-cd-apps.yaml". Each entry of the file has the following required properties:

- name: Name of the application inside Argo CD. Example: monitoring.
- repoURL: URL of the repo that Argo CD will connect to. Example:: https://github.com/ajaen4/eks-complete-cicd.
- path: path where the Kubernetes manifests are located. Example: k8s/dist.
- branch: specific branch of the repo to target. Example: main.

Once you have filled the necessary entries we can move on to deploy the EKS cluster.

### 2. Pulumi deployment

It is assumed that you already have an AWS account and have configured credentials. In this section we will deploy all the infrastructure related to AWS and the Argo CD application to be able to target other Kubernetes apps. Once you have installed Pulumi, we can start the deployment with the following command:

```bash
pulumi up
```

You will see a dialog with all the resources that will be deployed. Select "yes" to proceed.

### 3. Argo CD

Once the deployment finishes we need to register the eks cluster credentials in our computer, the command to do so is the following:

```bash
aws eks update-kubeconfig --name <cluster-name> --region <aws-region>
```

The next step is to connect to our Argo CD application. The simplest way is to open a connection from our local computer to the service. The Argo CD application has been deployed in the "cicd" namespace, so to check the services we need to run:

```bash
kubectl get svc -n cicd
```

You should see something like:
```bash
argo-cd-argocd-applicationset-controller
argo-cd-argocd-dex-server
argo-cd-argocd-redis
argo-cd-argocd-repo-server
argo-cd-argocd-server
```

The service we are looking for is the last one. To open a connection we must run:

```bash
kubectl port-forward svc/<service-name> <local-port>:80 -n cicd
```

Be sure to choose a port number between 1024 to 65535 to avoid reserved ranges.

Once the connection has been stablished you can now navigate in your browser to https://localhost:<local-port>. You should see the Argo CD UI. We now need to access the password to be able to log in. Argo CD creates a secret in Kubernetes to store this password, run the following commands to see all secrets:

```bash
kubectl get secret -n cicd
```

You should see something like:

```bash
argocd-initial-admin-secret
argocd-notifications-secret
argocd-redis
argocd-secret
```

The one we are looking for is the first one. It is important to note that the secret is base64 encoded, so the access it and decode it we need to run:

```bash
kubectl get secret <secret-name> -n cicd -o json | jq -r '.data.password' | base64 --decode
```

You can now use the password shown and the username "admin" to log in.

### 4. Deploy Kubernetes apps

We now need to deploy the different Kubernetes apps we have defined in the "argo-cd-apps.yaml". In this example our app is in the k8s folder and it's a CDK8s application. CDK8s is just a tool to be able to define Kubernetes manifests with a general purpose programming language (Python, Go...) instead of YAML. We just need to navigate to the folder and generate our manifests:

```bash
cd k8s
cdk8s import
cdk8s synth
```

This will generate all our Kubernetes manifests in the dist/ folder (which is the folder where our sample Argo config is targeting). Once the command is finished we can navigate to the applications in the Argo CD UI, you should see an application defined which has a "Missing" and "OutOfSync" status. We just need to click on "SYNC" for our application to be deployed.

This may take a while. Once finished all our infra and applications have been deployed!


## License

MIT License - Copyright (c) 2024 The eks-complete-cicd Authors.
