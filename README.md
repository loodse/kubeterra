# kubeterra
Simple Kubernetes Terraform integration - Manage your Terraform environment with
Kubernetes!

## About & Motivation

Kubeterra was created out of necessity for automation of managing cloud
resources in on-demand manner, where configurations are defined as terraform
modules.

Kubeterra itself is a controller manager that run on kubernetes and operating on
CustomResources.

### API Stability
API domain: kubeterra.io
API Group: terraform
Latest API Version: v1alpha1

Current API version `terraform.kubeterra.io/v1alpha1` is considered an alpha
version and most likely will be changed.

## Architecture

`terraform.kubeterra.io/v1alpha1` defines following objects.
* `TerraformConfiguration` — Defines terraform configuration source (read
  `main.tf`), including all variables, resources, locals and outputs. In
  addition contains references to ENV variables, secrets and volumes to mount
  that are needed in order to successfully run terraform apply.
* `TerraformState` — Contains terraform state translated here over kubeterra
  remove http backend.
* `TerraformPlan` — Contains snapshot of `TerraformConfiguration` that will be
  planned or applied.
* `TerraformLog` - Contains logs of terraform container after running
  `TerraformPlan`.

>Note: Users of kubeterra only need to create `TerraformConfiguration`, other
objects will be created by controller manager.

The intended workflow sequence looks like this:
* Actor creates `TerraformConfiguration`
* Kubeterra in response creates a pod with `terraform plan` or `terraform apply`
  container.
* Terraform container has possible configurations such as terraform config
  itself, volumes, environments variables, etc are mounted to this container.
* Kubeterra automatically run a sidecar container that provides [terraform http
  backend API](https://www.terraform.io/docs/backends/types/http.html), that's
  able to "proxy" terraform state back to cluster in `TerraformState` form.
* In addition to normal `main.tf` file (created from definition configured in
  TerraformConfiguration), `httpbackend.tf` is generated that will automatically
  instruct terraform to use httpbacked and point it towards "httpbackend"
  sidecar (this most likely will change, see issue #14).
* Once terraform container is finished, the whole pod is being removed, logs are
  saved.

## Deploy

Up to date release manifests in form of static YAML (that includes CRDs and RBAC
rules) are located in
[deploy/manifests/deploy.yaml](deploy/manifests/deploy.yaml).

Per release manifests can be found at
[releases](https://github.com/loodse/kubeterra/releases) in `Assets` attached to
every release.

## Usage

Standard deployment manifest defines RBAC rules only for `kubeterra-system`
namespace. All objects should be created there.

### Types shortcuts

For every defined type there a shortcut:
* `TerraformConfiguration` — `tfconfig`
* `TerraformState` — `tfstate`
* `TerraformPlan` — `tfplan`
* `TerraformLog` — `tflog`

So you can type something like

```shell
kubectl get tfconfig
```

### Simples example:

```yaml
apiVersion: terraform.kubeterra.io/v1alpha1
kind: TerraformConfiguration
metadata:
  name: random1
  namespace: kubeterra-system
spec:
  autoApprove: true
  configuration: |
    resource "random_id" "rand" {
      byte_length = 4
    }
```

Once this object is created kubeterra will terraform apply it and state will be
saved into random1 `TerraformState` object.

To pull state locally:

```shell
kubectl get tfstate random1 -ojson | jq '.spec.state'
```

To pull only outputs locally:

```shell
kubectl get tfstate random1 -ojson | jq '.spec.state.outputs'
```

More advanced example is defined in [/config/samples/kubeone](https://github.com/loodse/kubeterra/tree/master/config/samples/kubeone)

See also: [TerraformConfigurationSpec API definition](https://github.com/loodse/kubeterra/blob/79ecb7c955255592172fb93a372db25c1e4fa7a6/api/v1alpha1/types.go#L73)

## Caveats & Limitations

* For RBAC reason currently by default `TerraformConfiguration` are limited to
  `kubeterra-system` namespace.
* `terraform destroy` is currently not supported (see issue #15).
