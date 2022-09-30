
locals {
  business_unit           = "k8s-agent-e2e-test-private"
  safe_kubernetes_version = replace(var.kubernetes_version, ".", "-")
  cluster_name            = "${var.name_prefix}-${var.safe_kubernetes_version}-${local.business_unit}-zonal-cluster"
  network_name            = "${var.name_prefix}-${var.safe_kubernetes_version}-${local.business_unit}-network"
  subnet_name             = "${var.name_prefix}-${var.safe_kubernetes_version}-${local.business_unit}-subnet"
  master_auth_subnetwork  = "${var.name_prefix}-${var.safe_kubernetes_version}-${local.business_unit}-master-subnet"
  pods_range_name         = "${var.name_prefix}-${var.safe_kubernetes_version}-${local.business_unit}-ip-range-pods"
  svc_range_name          = "${var.name_prefix}-${var.safe_kubernetes_version}-${local.business_unit}-ip-range-svc"
  subnet_names            = [for subnet_self_link in module.gcp-network.subnets_self_links : split("/", subnet_self_link)[length(split("/", subnet_self_link)) - 1]]
}

data "google_client_config" "default" {}

provider "kubernetes" {
  host                   = "https://${module.gke.endpoint}"
  token                  = data.google_client_config.default.access_token
  cluster_ca_certificate = base64decode(module.gke.ca_certificate)
}

module "gke" {
  source                  = "./modules/private-cluster/"

  project_id              = var.project_id
  name                    = local.cluster_name
  kubernetes_version      = var.kubernetes_version
  regional                = false
  region                  = var.region
  zones                   = var.zones
  network                 = module.gcp-network.network_name
  subnetwork              = local.subnet_names[index(module.gcp-network.subnets_names, local.subnet_name)]
  ip_range_pods           = local.pods_range_name
  ip_range_services       = local.svc_range_name
  create_service_account  = true
  service_account         = ""
  enable_private_endpoint = true
  enable_private_nodes    = true
  master_ipv4_cidr_block  = "172.16.0.0/28"

  master_authorized_networks = [
    {
      cidr_block   = "10.60.0.0/17"
      display_name = "VPC"
    },
  ]
}
