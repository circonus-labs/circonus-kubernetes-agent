variable "network_name" {
  type = string
  description = "the name of the vpc"
}

variable "project_id" {
  type = string
  description = "The project ID to host the network in"
}

variable "region" {
  type = string
  description = "The region to use"
}

variable "subnet_name" {
  type = string
  description = "the name of the subnet"
}

variable "master_subnet_name" {
  type = string
  description = "the name of the master subnet"
}

variable "pods_range_name" {
  type = string 
  description = "the name of the pods range"
}

variable "svc_range_name" {
  type = string 
  description = "the name of the service range"
}

