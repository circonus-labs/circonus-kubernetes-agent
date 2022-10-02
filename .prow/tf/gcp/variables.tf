variable "kubernetes_version" {
  type        = string
  description = "The version of kubernetes to deploy"
}

variable "release_channel" {
  type        = string
  description = "The release channel of kubernetes to deploy"
}

variable "name_prefix" {
  type        = string
  description = "prefix to apply to all resource names"
  default     = "manual"
}

variable "project_id" {
  type        = string
  description = "The ID of the project to create resources in"
}

variable "region" {
  type        = string
  description = "The region to use"
}

variable "main_zone" {
  type        = string
  description = "The zone to use as primary"
}

variable "cluster_node_zones" {
  type        = list(string)
  description = "The zones where Kubernetes cluster worker nodes should be located"
}

