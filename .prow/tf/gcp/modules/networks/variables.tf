variable "project_id" {
  type        = string
  description = "The project ID to host the network in"
}

variable "region" {
  type        = string
  description = "The region to use"
}

variable "network_name" {
  type        = string
  description = "The name of the network to create"
}

variable "subnet_name" {
  type        = string
  description = "The name of the subnet to create"
}

variable "subnet_cidr" {
  type        = string
  description = "The CIDR of the subnet"
}

