#### What is `kubeterra` ?

`kubeterra` is a tool that manages your [Terraform](https://www.terraform.io/) environment with kubernetes.

It provides kubernetes CRDs that match Terraform objects. In a nutshell `kubeterra` is a kubernetes controller.



##### Sample Resources:

You can find sample manifests for resource in `kubeterra/config/samples/` directory.



##### Run the tests

`$ make test`



##### Generate binary
`$ make manager`



##### Install CRDs into cluster

`$ make install`



##### Deploy the controller manager manifests to the cluster

`$ make deploy`



##### Run the controller on node

`$ make run`



##### Create a docker image

`$ make docker-build IMG=<img-name>`



##### Push the docker image to a configured container registry

`$ make docker-push IMG=<img-name>`
