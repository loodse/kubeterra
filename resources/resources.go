/*

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package resources

const (
	TerraformApplyAutoApproveScript = `
terraform init -no-color
exec terraform apply -no-color -input=false -auto-approve
	`

	TerraformPlanScript = `
terraform init -no-color
exec terraform plan -no-color -input=false
`

	TerraformHTTPBackendConfig = `
terraform {
	required_version = ">= 0.12"
	backend "http" {
		address        = "http://localhost:8081/"
		lock_address   = "http://localhost:8081/"
		unlock_address = "http://localhost:8081/"
	}
}
`

	LinkedTerraformConfigMapAnnotation = "linked-terraform-config-map"
)
