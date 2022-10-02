output "bastion_open_tunnel_command" {
  description = "Command that opens an SSH tunnel to the Bastion instance."
  value       = "${module.bastion.ssh} -f tail -f /dev/null"
}

output "get_credentials" {
  description = "Gcloud get-credentials command"
  value       = format("gcloud container clusters get-credentials --project %s --region %s --internal-ip %s", var.project_id, var.region, local.cluster_name)
}

output "kubectl_alias_command" {
  description = "Command that creates an alias for kubectl using Bastion as proxy. Bastion ssh tunnel must be running."
  value       = "alias kube='${module.bastion.kubectl_command}'"
}
