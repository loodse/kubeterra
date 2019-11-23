# Architecture

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
  
### API Stability
API domain: kubeterra.io
API Group: terraform
Latest API Version: v1alpha1

Current API version `terraform.kubeterra.io/v1alpha1` is considered an alpha
version and most likely will be changed.