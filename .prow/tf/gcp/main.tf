locals {
  business_unit = "k8s-agent-e2e"

  safe_kubernetes_version = replace(var.kubernetes_version, ".", "-")

  bastion_name       = "${var.name_prefix}-${local.safe_kubernetes_version}-${local.business_unit}-bastion"
  cluster_name       = "${var.name_prefix}-${local.safe_kubernetes_version}-${local.business_unit}-zonal-cluster"
  network_name       = "${var.name_prefix}-${local.safe_kubernetes_version}-${local.business_unit}-network"
  subnet_name        = "${var.name_prefix}-${local.safe_kubernetes_version}-${local.business_unit}-subnet"
  master_subnet_name = "${var.name_prefix}-${local.safe_kubernetes_version}-${local.business_unit}-master-subnet"
  pods_range_name    = "${var.name_prefix}-${local.safe_kubernetes_version}-${local.business_unit}-ip-range-pods"
  svc_range_name     = "${var.name_prefix}-${local.safe_kubernetes_version}-${local.business_unit}-ip-range-svc"
  registry_name      = "${var.name_prefix}-${local.safe_kubernetes_version}-${local.business.unit}"

  subnet_cidr                    = "10.10.0.0/16"
  cluster_master_ip_cidr_range   = "10.100.100.0/28"
  cluster_pods_ip_cidr_range     = "10.101.0.0/16"
  cluster_services_ip_cidr_range = "10.102.0.0/16"
}

provider "google" {
  project = var.project_id
  region  = var.region
  zone    = var.main_zone
}

module "google_networks" {
  source = "./modules/networks"

  project_id = var.project_id
  region     = var.region

  network_name = local.network_name
  subnet_name  = local.subnet_name
  subnet_cidr  = local.subnet_cidr
}

module "google_kubernetes_cluster" {
  source = "./modules/kubernetes_cluster"

  project_id = var.project_id
  region     = var.region

  cluster_name               = local.cluster_name
  kubernetes_version         = var.kubernetes_version
  release_channel            = var.release_channel
  node_zones                 = var.cluster_node_zones
  network_name               = module.google_networks.network.name
  subnet_name                = module.google_networks.subnet.name
  master_ipv4_cidr_block     = local.cluster_master_ip_cidr_range
  pods_ipv4_cidr_block       = local.cluster_pods_ip_cidr_range
  services_ipv4_cidr_block   = local.cluster_services_ip_cidr_range
  authorized_ipv4_cidr_block = "${module.bastion.ip}/32"
}

module "bastion" {
  source = "./modules/bastion"

  project_id   = var.project_id
  region       = var.region
  zone         = var.main_zone
  bastion_name = local.bastion_name
  network_name = module.google_networks.network.name
  subnet_name  = module.google_networks.subnet.name
}

resource "google_artifact_registry_repository" "k8s_agent_registry" {
  location      = var.region
  repository_id = local.registry_name
  description   = "container image registry for the kubernetes agent"
  format        = "DOCKER"
}

