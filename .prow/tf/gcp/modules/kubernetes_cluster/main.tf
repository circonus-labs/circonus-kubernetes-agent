locals {

  old_node_metadata_config_mapping = { GKE_METADATA_SERVER = "GKE_METADATA", GCE_METADATA = "EXPOSE" }
  cluster_node_metadata_config = var.node_metadata == "UNSPECIFIED" ? [] : [{
    mode = lookup(local.old_node_metadata_config_mapping, var.node_metadata, var.node_metadata)
  }]

  workload_identity_enabled = true
  identity_namespace        = "enabled"
  cluster_workload_identity_config = !local.workload_identity_enabled ? [] : local.identity_namespace == "enabled" ? [{
    workload_pool = "${var.project_id}.svc.id.goog" }] : [{ workload_pool = local.identity_namespace
  }]
}

resource "google_container_cluster" "product_cluster" {
  name     = var.cluster_name
  location = var.region

  min_master_version = var.kubernetes_version
  # We can't create a cluster with no node pool defined, but we want to only use
  # separately managed node pools. So we create the smallest possible default
  # node pool and immediately delete it.
  remove_default_node_pool = true
  initial_node_count       = 1

  ip_allocation_policy {
    cluster_ipv4_cidr_block  = var.pods_ipv4_cidr_block
    services_ipv4_cidr_block = var.services_ipv4_cidr_block
  }

  network    = var.network_name
  subnetwork = var.subnet_name

  logging_service    = "logging.googleapis.com/kubernetes"
  monitoring_service = "monitoring.googleapis.com/kubernetes"

  maintenance_policy {
    daily_maintenance_window {
      start_time = "02:00"
    }
  }

  master_auth {
    client_certificate_config {
      issue_client_certificate = true
    }
  }

  dynamic "master_authorized_networks_config" {
    for_each = var.authorized_ipv4_cidr_block != null ? [var.authorized_ipv4_cidr_block] : []
    content {
      cidr_blocks {
        cidr_block   = master_authorized_networks_config.value
        display_name = "External Control Plane access"
      }
    }
  }

  private_cluster_config {
    enable_private_endpoint = true
    enable_private_nodes    = true
    master_ipv4_cidr_block  = var.master_ipv4_cidr_block
  }

  release_channel {
    channel = var.release_channel
  }

  addons_config {
    // Enable network policy (Calico)
    network_policy_config {
      disabled = false
    }
  }

  /* Enable network policy configurations (like Calico).
  For some reason this has to be in here twice. */
  network_policy {
    enabled = "true"
  }

  # TODO
  dynamic "workload_identity_config" {
    for_each = local.cluster_workload_identity_config

    content {
      workload_pool = workload_identity_config.value.workload_pool
    }
  }
}

resource "google_container_node_pool" "product_cluster_linux_node_pool" {
  name           = substr("${google_container_cluster.product_cluster.name}--linux-node-pool", 0, 39)
  location       = google_container_cluster.product_cluster.location
  node_locations = var.node_zones
  cluster        = google_container_cluster.product_cluster.name
  node_count     = 1
  version        = google_container_cluster.product_cluster.min_master_version

  autoscaling {
    max_node_count = 5
    min_node_count = 1
  }
  max_pods_per_node = 100

  management {
    auto_repair  = true
    auto_upgrade = true
  }

  node_config {
    preemptible  = true
    disk_size_gb = 10

    service_account = google_service_account.cluster_service_account.email

    oauth_scopes = [
      "https://www.googleapis.com/auth/devstorage.read_only",
      "https://www.googleapis.com/auth/logging.write",
      "https://www.googleapis.com/auth/monitoring",
      "https://www.googleapis.com/auth/servicecontrol",
      "https://www.googleapis.com/auth/service.management.readonly",
      "https://www.googleapis.com/auth/trace.append",
    ]

    labels = {
      cluster = google_container_cluster.product_cluster.name
    }

    shielded_instance_config {
      enable_secure_boot = true
    }

    dynamic "workload_metadata_config" {
      for_each = local.cluster_node_metadata_config

      content {
        mode = workload_metadata_config.value.mode
      }
    }

    metadata = {
      // Set metadata on the VM to supply more entropy.
      google-compute-enable-virtio-rng = "true"
      // Explicitly remove GCE legacy metadata API endpoint.
      disable-legacy-endpoints = "true"
    }
  }

  upgrade_settings {
    max_surge       = 1
    max_unavailable = 1
  }
}
