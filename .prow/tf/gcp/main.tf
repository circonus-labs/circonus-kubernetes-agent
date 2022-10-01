
locals {
  business_unit           = "k8s-agent-e2e"
  main_zone               = var.zones[0]
  safe_kubernetes_version = replace(var.kubernetes_version, ".", "-")
  bastion_name            = "${var.name_prefix}-${local.safe_kubernetes_version}-${local.business_unit}-bastion"
  cluster_name            = "${var.name_prefix}-${local.safe_kubernetes_version}-${local.business_unit}-zonal-cluster"
  network_name            = "${var.name_prefix}-${local.safe_kubernetes_version}-${local.business_unit}-network"
  subnet_name             = "${var.name_prefix}-${local.safe_kubernetes_version}-${local.business_unit}-subnet"
  master_subnet_name      = "${var.name_prefix}-${local.safe_kubernetes_version}-${local.business_unit}-master-subnet"
  pods_range_name         = "${var.name_prefix}-${local.safe_kubernetes_version}-${local.business_unit}-ip-range-pods"
  svc_range_name          = "${var.name_prefix}-${local.safe_kubernetes_version}-${local.business_unit}-ip-range-svc"
  subnet_names            = [for subnet_self_link in module.network.subnets_self_links : split("/", subnet_self_link)[length(split("/", subnet_self_link)) - 1]]

  subnet_cidr            = "10.0.0.0/17"
  master_authorized_cidr = "10.60.0.0/17"
  pods_cidr              = "192.168.0.0/18"
  svc_cidr               = "192.168.64.0/18"
  master_ipv4_cidr_block = "172.16.0.0/28"
}

data "google_client_config" "default" {}

provider "google" {
  project = var.project_id
  region  = var.region
  zone    = local.main_zone
}

provider "kubernetes" {
  host                   = "https://${module.gke.endpoint}"
  token                  = data.google_client_config.default.access_token
  cluster_ca_certificate = base64decode(module.gke.ca_certificate)
}

module "network" {
  source                 = "./modules/network/"
  network_name           = local.network_name
  project_id             = var.project_id
  region                 = var.region
  subnet_name            = local.subnet_name
  subnet_cidr            = local.subnet_cidr
  master_subnet_name     = local.master_subnet_name
  master_authorized_cidr = local.master_authorized_cidr
  pods_range_name        = local.pods_range_name
  pods_cidr              = local.pods_cidr
  svc_range_name         = local.svc_range_name
  svc_cidr               = local.svc_cidr
}

module "bastion" {
  source       = "./modules/bastion/"
  bastion_name = local.bastion_name
  project_id   = var.project_id
  region       = var.region
  zone         = local.main_zone
  network_name = local.network_name
  subnet_name  = local.subnet_names[index(module.network.subnets_names, local.subnet_name)]
}

module "gke" {
  source                  = "./modules/private-cluster/"
  name                    = local.cluster_name
  project_id              = var.project_id
  kubernetes_version      = var.kubernetes_version
  release_channel         = var.release_channel
  regional                = false
  region                  = var.region
  zones                   = var.zones
  network                 = local.network_name
  subnetwork              = local.subnet_names[index(module.network.subnets_names, local.subnet_name)]
  ip_range_pods           = local.pods_range_name
  ip_range_services       = local.svc_range_name
  create_service_account  = true
  service_account         = ""
  enable_private_endpoint = true
  enable_private_nodes    = true
  master_ipv4_cidr_block  = local.master_ipv4_cidr_block

  master_authorized_networks = [
    {
      cidr_block   = local.master_authorized_cidr
      display_name = "VPC"
    },
  ]

  node_pools = [
    {
      name         = "default-node-pool",
      auto_repair  = true
      auto_upgrade = true
    },
  ]

}
