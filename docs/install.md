---
title: Install
layout: docwithnav
---

Starting from version 0.3.x, Service Catalog uses [Admission Webhooks](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/#what-are-admission-webhooks)
to manage custom resources. It uses [Additional Printer Columns](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#additional-printer-columns)
so you can use `kubectl` to interact with Service Catalog.

The rest of this document details how to:

- Set up Service Catalog on your cluster
- Interact with the Service Catalog API

# Step 1 - Prerequisites

## Kubernetes Version

Service Catalog requires a Kubernetes cluster v1.13 or later. You'll also need a
[Kubernetes configuration file](https://kubernetes.io/docs/tasks/access-application-cluster/configure-access-multiple-clusters/)
installed on your host. You need this file so you can use `kubectl` and
[`helm`](https://helm.sh) to communicate with the cluster. Many Kubernetes installation
tools and/or cloud providers will set this configuration file up for you. Please
check with your tool or provider for details.

### `kubectl` Version

Most interaction with the service catalog system is achieved through the
`kubectl` command line interface. As with the cluster version, Service Catalog
requires `kubectl` version 1.13 or newer.

First, check your version of `kubectl`:

```console
kubectl version
```

Ensure that the server version and client versions are both `1.13` or above.

If you need to upgrade your client, follow the
[installation instructions](https://kubernetes.io/docs/tasks/kubectl/install/)
to get a new `kubectl` binary.

For example, run the following command to get an up-to-date binary on Mac OS:

```console
curl -LO https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/darwin/amd64/kubectl
chmod +x ./kubectl
```

## In-Cluster DNS

You'll need a Kubernetes installation with in-cluster DNS enabled. Most popular
installation methods will automatically configure in-cluster DNS for you:

- [Minikube](https://github.com/kubernetes/minikube)
- [`hack/local-up-cluster.sh`](https://github.com/kubernetes/kubernetes/blob/master/hack/local-up-cluster.sh)
(in the Kubernetes repository)
- Most cloud providers

## Storage

Service Catalog uses CRDs to store information.

## Helm

You'll install Service Catalog with [Helm](http://helm.sh/), and you'll need
v3.4.0 or newer for that. See the steps below to install.

## Helm Repository Setup

Service catalog uses OCI to store charts. Here are two OCI addresses, which store testing and stable versions respectively.

* `helm install catalog oci://registry.drycc.cc/charts/catalog`
* `helm install catalog oci://registry.drycc.cc/charts-testing/catalog`

## RBAC

Your Kubernetes cluster must have
[RBAC](https://kubernetes.io/docs/reference/access-authn-authz/rbac/) enabled to use
Service Catalog.

Like in-cluster DNS, many installation methods should enable RBAC for you.

### Minikube

When using Minikube v0.25 or older, you must run Minikube with RBAC explicitly
enabled:

```
minikube start --extra-config=apiserver.Authorization.Mode=RBAC
```

When using Minikube v0.26+, run the following command:

```
minikube start
```

With Minikube v0.26+, do not specify `--extra-config`. The
flag has since been changed to `--extra-config=apiserver.authorization-mode` and
Minikube now uses RBAC by default. Specifying the older flag may cause the
start command to hang.

### `hack/local-cluster-up.sh`

If you are using the
[`hack/local-up-cluster.sh`](https://github.com/kubernetes/kubernetes/blob/master/hack/local-up-cluster.sh)
script in the Kubernetes core repository, start your cluster with this command:

```console
AUTHORIZATION_MODE=Node,RBAC hack/local-up-cluster.sh -O
```

### Cloud Providers

Many cloud providers set up new clusters with RBAC enabled for you. Please
check with your provider's documentation for details.

# Step 2 - Install Service Catalog

Now that your cluster and Helm are configured properly, installing
Service Catalog is simple:

```console
helm install catalog drycc/catalog --namespace catalog --create-namespace
```

# Installing the Service Catalog CLI

Follow the appropriate instructions for your operating system to install svcat. The binary can be used by itself, or as a kubectl plugin.

The snippets below install the latest version of svcat. We publish binaries for our release builds.

## MacOS with Homebrew

```
brew update
brew install kubernetes-service-catalog-client
```

## MacOS

```
curl -sLO https://download.svcat.sh/cli/latest/darwin/amd64/svcat
chmod +x ./svcat
mv ./svcat /usr/local/bin/
svcat version --client
```

## Linux

```
curl -sLO https://download.svcat.sh/cli/latest/linux/amd64/svcat
chmod +x ./svcat
mv ./svcat /usr/local/bin/
svcat version --client
```

## Windows

The PowerShell snippet below adds a directory to your PATH for the current session only.
You will need to find a permanent location for it and add it to your PATH.

```
iwr 'https://download.svcat.sh/cli/latest/windows/amd64/svcat.exe' -UseBasicParsing -OutFile svcat.exe
mkdir -f ~\bin
Move-Item -Path svcat.exe  -Destination ~\bin
$env:PATH += ";${pwd}\bin"
svcat version --client
```

## Manual
1. Download the appropriate binary for your operating system:
    * macOS: https://download.svcat.sh/cli/latest/darwin/amd64/svcat
    * Windows: https://download.svcat.sh/cli/latest/windows/amd64/svcat.exe
    * Linux: https://download.svcat.sh/cli/latest/linux/amd64/svcat
1. Make the binary executable.
1. Move the binary to a directory on your PATH.

## Plugin
To use svcat as a plugin, run the following command after downloading:

```console
$ ./svcat install plugin
Plugin has been installed to ~/.kube/plugins/svcat. Run kubectl plugin svcat --help for help using the plugin.
```

When operating as a plugin, the commands are the same with the addition of the global
kubectl configuration flags. One exception is that boolean flags aren't supported
when running in plugin mode, so instead of using `--flag` you must specify a value `--flag=true`.
