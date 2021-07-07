output "public_ip" {
  value = "${metal_device.web1.network.0.address}"
}

output "static_terraform_output" {
  description = <<EOD
This output is used as an attribute in the inspec_attributes control
EOD

  value = "static terraform output"
}

output "terraform_state" {
  description = "This output is used as an attribute in the state_file control"

  value = <<EOV
${path.cwd}/terraform.tfstate.d/${terraform.workspace}/terraform.tfstate
EOV
}