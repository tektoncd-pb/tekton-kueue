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
Otherwise you can install it with:

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

Resource requests can be specified by placing annotations on the PipelineRun resource. The available annotations are:

- `kueue.konflux-ci.dev/requests-cpu`
- `kueue.konflux-ci.dev/requests-memory`
- `kueue.konflux-ci.dev/requests-storage`
- `kueue.konflux-ci.dev/requests-ephemeral-storage`

By default, a special resource called `tekton.dev/pipelineruns` is added to the [Workload] with the value of 1.
This resource can be used for controlling the number of PipelineRuns that can be executed concurrently.

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

## Command Line Interface

The `tekton-kueue` binary provides several subcommands:

### `mutate` - Apply PipelineRun Mutations

The `mutate` subcommand allows you to test and preview how PipelineRuns will be modified by the webhook before deploying them to the cluster. This is useful for:

- Testing CEL expressions and mutation logic
- Previewing changes before applying them
- Debugging configuration issues
- Validating PipelineRun transformations

#### Usage

```sh
tekton-kueue mutate --pipelinerun-file <path> --config-dir <path>
```

#### Parameters

- `--pipelinerun-file`: Path to the file containing the PipelineRun definition (required)
- `--config-dir`: Path to the directory containing the configuration file (required)
- `--zap-log-level`: Set logging level (debug, info, error)

#### Example

Create a test PipelineRun file:

```yaml
# test-pipelinerun.yaml
apiVersion: tekton.dev/v1
kind: PipelineRun
metadata:
  name: test-pipeline-run
  namespace: default
spec:
  pipelineRef:
    name: test-pipeline
  workspaces:
    - name: shared-workspace
      emptyDir: {}
```

Create a configuration file:

```yaml
# config/config.yaml
queueName: "test-queue"
cel:
  expressions:
    - 'annotation("tekton.dev/mutated-by", "tekton-kueue")'
    - 'label("environment", "test")'
    - '[annotation("build.tekton.dev/timestamp", "2025-01-01T00:00:00Z"), label("app", "test-app")]'
```

Run the mutation:

```sh
tekton-kueue mutate --pipelinerun-file test-pipelinerun.yaml --config-dir config/
```

This will output the mutated PipelineRun with:
- Applied annotations from CEL expressions
- Applied labels from CEL expressions  
- Queue name label (`kueue.x-k8s.io/queue-name`)
- Status set to `PipelineRunPending`

#### CEL Expression Examples

The configuration supports [CEL (Common Expression Language)](https://github.com/google/cel-spec) expressions for dynamic mutations:

```yaml
cel:
  expressions:
    # Single annotation
    - 'annotation("tekton.dev/pipeline", pipelineRun.metadata.name)'
    
    # Single label
    - 'label("environment", "production")'
    
    # Multiple mutations in one expression
    - '[annotation("build.time", "2025-01-01T00:00:00Z"), label("team", "platform")]'
    
    # Conditional mutations based on PipelineRun data
    - 'pipelineRun.metadata.namespace == "prod" ? label("priority", "high") : label("priority", "normal")'
    
    # Multiline CEL expression for multiple mutations
    # This expression applies several annotations and labels in one go
    - |
      [
        annotation("tekton.dev/pipeline", pipelineRun.metadata.name),
        annotation("tekton.dev/namespace", pipelineRun.metadata.namespace),
        label("app", "tekton-pipeline"),
        label("version", "v1"),
        pipelineRun.metadata.namespace == "production" ? 
          label("environment", "prod") : 
          label("environment", "dev")
      ]
```

**What the multiline expression does:**

1. **Creates annotations** from PipelineRun metadata (pipeline name and namespace)
2. **Adds standard labels** that apply to all PipelineRuns (`app` and `version`)
3. **Applies conditional logic** to set the `environment` label based on the namespace
4. **Uses YAML multiline syntax** (`|`) to make complex expressions readable
5. **Returns a list** of mutations that are all applied together

For a PipelineRun named `my-pipeline` in namespace `production`, this would add:
- Annotations: `tekton.dev/pipeline: my-pipeline`, `tekton.dev/namespace: production`
- Labels: `app: tekton-pipeline`, `version: v1`, `environment: prod`

### Other Subcommands

- `controller` - Run the tekton-kueue controller
- `webhook` - Run the admission webhook server

## Project Distribution

The project is built by [Konflux]. Images are published to [quay.io/konflux-ci/tekton-queue](quay.io/konflux-ci/tekton-queue)

## Contributing

**NOTE:** Run `make help` for more information on all potential `make` targets.

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)


[Tekton]: <https://tekton.dev/> "Tekton"
[Kueue]: <https://kueue.sigs.k8s.io/> "Kueue"
[Konflux]: <https://konflux-ci.dev/> "Konflux"
[ResourceFlavor]: <https://kueue.sigs.k8s.io/docs/concepts/resource_flavor/> "ResourceFlavor"
[ClusterQueue]: <https://kueue.sigs.k8s.io/docs/concepts/cluster_queue/> "ClusterQueue"
[LocalQueue]: <https://kueue.sigs.k8s.io/docs/concepts/local_queue/> "LocalQueue"
[Workload]: <https://kueue.sigs.k8s.io/docs/concepts/workload/> "Workload"
