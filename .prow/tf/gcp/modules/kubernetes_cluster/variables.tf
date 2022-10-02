variable "project_id" {
  type        = string
  description = "The project ID to host the network in"
}

variable "region" {
  type        = string
  description = "The region to use"
}

variable "cluster_name" {
  type        = string
  description = "The name of the cluster"
}

variable "kubernetes_version" {
  type        = string
  description = "The version of kubernetes to use"
}

variable "release_channel" {
  type        = string
  description = "The channel of kubernetes versions to use"
}

variable "node_zones" {
  type        = list(string)
  description = "The zones where worker nodes are located"
}

variable "network_name" {
  type        = string
  description = "The name of the app VPC"
}

variable "subnet_name" {
  type        = string
  description = "The name of the app subnet"
}

variable "pods_ipv4_cidr_block" {
  type        = string
  description = "The CIDR block to use for pod IPs"
}

variable "services_ipv4_cidr_block" {
  type        = string
  description = "The CIDR block to use for the service IPs"
}

variable "authorized_ipv4_cidr_block" {
  type        = string
  description = "The CIDR block where HTTPS access is allowed from"
  default     = null
}

variable "master_ipv4_cidr_block" {
  type        = string
  description = "The /28 CIDR block to use for the master IPs"
}

variable "node_metadata" {
  description = "Specifies how node metadata is exposed to the workload running on the node"
  default     = "GKE_METADATA"
  type        = string

  validation {
    condition     = contains(["GKE_METADATA", "GCE_METADATA", "UNSPECIFIED", "GKE_METADATA_SERVER", "EXPOSE"], var.node_metadata)
    error_message = "The node_metadata value must be one of GKE_METADATA, GCE_METADATA or UNSPECIFIED."
  }
}

