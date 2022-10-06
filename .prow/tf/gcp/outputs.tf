output "project_id" {
  description = "the project id"
  value       = "${var.project_id}"
}

output "region" {
  description = "the region"
  value       = "${var.region}"
}

output "name_prefix" {
  description = "the prefix applied to all resource names"
  value       = "${var.name_prefix}"
}

output "cluster_name" {
  description = "the name of the cluster"
  value       = "${local.cluster_name}"
}

output "bastion_open_tunnel_command" {
  description = "Command that opens an SSH tunnel to the Bastion instance."
  value       = "${module.bastion.ssh} -f tail -f /dev/null"
}

output "bastion_name" {
  description = "the name of the bastion instance"
  value       = module.bastion.name
}

output "bastion_zone" {
  description = "the zone the bastion instance is deployed in."
  value       = module.bastion.zone
}

output "get_credentials" {
  description = "Gcloud get-credentials command"
  value       = format("gcloud container clusters get-credentials --project %s --region %s --internal-ip %s", var.project_id, var.region, local.cluster_name)
}

output "kubectl_alias_command" {
  description = "Command that creates an alias for kubectl using Bastion as proxy. Bastion ssh tunnel must be running."
  value       = "alias kube='${module.bastion.kubectl_command}'"
}
