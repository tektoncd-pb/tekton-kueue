# tekton-kueue

Controller for integrating [Tekton] with [Kueue].

## Description

The controller enables [Kueue] to manage the scheduling of [Tekton] PipelineRuns.

## Getting Started

### Prerequisites
- kubectl version v1.11.3+.
- Access to a Kubernetes v1.11.3+ cluster.
- make
- Get familiar with [basic Kueue concept](https://kueue.sigs.k8s.io/docs/concepts/)

### To Deploy on the cluster

**Install CertManager:**

CertManager is required for providing certificates for the admission webhook.

```sh
make cert-manager
```

**Install Kueue:**

The controller currently supports kueue v0.10.x
If you already have kueue installed, make sure to enable it by adding `pipelineruns.tekton.dev` to the external frameworks.

```sh
make kueue
```

**Install Tekton:**
```sh
make tekton
```

**Deploy the Manager to the cluster with the image specified by `IMG`:**

```sh
make deploy IMG=quay.io/konflux-ci/tekton-kueue:latest
```

### To Uninstall

**UnDeploy the controller from the cluster:**

```sh
make undeploy
```

### Usage

The controller has an admission webhook that will associate any PipelineRun created
on the cluster with a `LocalQueue` named `pipelines-queue`.
The association is made using a label.

You can limit the admission webhook to act on [certain namespaces](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/#matching-requests-namespaceselector) by modifying config/webhook/manifests.yaml

In order to use [Kueue], you need to create (at least) the following resource:

- [ResourceFlavor]
- [ClusterQueue]
- [LocalQueue]

For convenience, we will provide a sample of those resources.

**Note:** Some of the resources are namespaced, so make sure to create a namespace for testing before applying the samples.

Create a namespace:

```sh
kubectl create namespace tekton-kueue-test
```

You can apply the samples using:

```sh
kubectl apply -n tekton-kueue-test -f config/samples/kueue/kueue-resources.yaml
```

By default, the controller will assume that each PipelineRun resource requests are of 1Gi of Memory.
This can be changed by placing annotations on the PipelineRun resource. The available annotations are:

- `kueue.konflux-ci.dev/requests-cpu`
- `kueue.konflux-ci.dev/requests-memory`
- `kueue.konflux-ci.dev/requests-storage`
- `kueue.konflux-ci.dev/requests-ephemeral-storage`

You are now ready to create PipelineRuns that will get scheduled by Kueue.
You can use the following PipelineRun definition (which prints a message to stdout after sleeping for several seconds):

```sh
kubectl create -n tekton-kueue-test -f config/samples/pipelines/pipeline.yaml
```

After creating the PipelineRun, You can see the status of the [ClusterQueue] by running

```sh
kubectl get -o yaml clusterqueue cluster-pipeline-queue
```

You can also see the [Workload] resource that the `tekton-kueue` controller created

```sh
kubectl get -n tekton-kueue-test workloads
```

If You'll try to create several PipelineRuns at one, you would see that some
of them get queued because the [ClusterQueue] resource reaches its resource limit.

## Project Distribution

The project is built by [Konflux]. Images are published to [quay.io/konflux-ci/tekton-queue](quay.io/konflux-ci/tekton-queue)

## Contributing

**NOTE:** Run `make help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)


[Tekton]: <https://tekton.dev/> "Tekton"
[Kueue]: <https://kueue.sigs.k8s.io/> "Kueue"
[Konflux]: <https://konflux-ci.dev/> "Konflux"
[ResourceFlavor]: <https://kueue.sigs.k8s.io/docs/concepts/resource_flavor/> "ResourceFlavor"
[ClusterQueue]: <https://kueue.sigs.k8s.io/docs/concepts/cluster_queue/> "ClusterQueue"
[LocalQueue]: <https://kueue.sigs.k8s.io/docs/concepts/local_queue/> "LocalQueue"
[Workload]: <https://kueue.sigs.k8s.io/docs/concepts/workload/> "Workload"
