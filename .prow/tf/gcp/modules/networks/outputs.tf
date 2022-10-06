output "network" {
  value       = google_compute_network.vpc
  description = "The VPC"
}

output "subnet" {
  value       = google_compute_subnetwork.subnet
  description = "The subnet"
}

