output "name" {
  value       = google_container_cluster.product_cluster.name
  description = "The Kubernetes cluster name."
}

output "service_account" {
  value       = 
  description = "The service account of the cluster."
}
