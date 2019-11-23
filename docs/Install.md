# Deploy & Usage

## Install

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
