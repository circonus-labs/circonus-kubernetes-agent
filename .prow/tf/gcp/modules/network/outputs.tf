output "subnets_names" {
  value       = module.gcp-network.subnets_names
  description = "The subnet names"
}

output "subnets_self_links" {
  value       = module.gcp-network.subnets_self_links
  description = "The subnbet self links"
}

