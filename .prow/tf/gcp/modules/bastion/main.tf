// Dedicated service account for the Bastion instance.
resource "google_service_account" "bastion" {
  account_id   = substr(var.bastion_name, 0, 27)
  display_name = "GKE Bastion Service Account"
}

// Allow access to the Bastion Host via SSH.
resource "google_compute_firewall" "bastion-ssh" {
  name          = format("%s-bastion-ssh", var.bastion_name)
  network       = var.network_name
  direction     = "INGRESS"
  project       = var.project_id
  source_ranges = ["0.0.0.0/0"] // TODO: Restrict further through dev vpn.

  allow {
    protocol = "tcp"
    ports    = ["22"]
  }

  target_tags = ["bastion"]
}

// The Bastion host.
resource "google_compute_instance" "bastion" {
  name         = var.bastion_name
  machine_type = "e2-micro"
  zone         = var.zone
  project      = var.project_id
  tags         = ["bastion"]

  boot_disk {
    initialize_params {
      image = "debian-cloud/debian-11"
    }
  }

  shielded_instance_config {
    enable_secure_boot          = true
    enable_vtpm                 = true
    enable_integrity_monitoring = true
  }

  // Install tinyproxy on startup.
  metadata_startup_script = <<SCRIPT
  sudo apt-get update -y
  sudo apt-get install -y tinyproxy
  SCRIPT

  network_interface {
    subnetwork = var.subnet_name


    access_config {
      // Not setting "nat_ip", use an ephemeral external IP.
      network_tier = "STANDARD"
    }
  }

  // Allow the instance to be stopped by Terraform when updating configuration.
  allow_stopping_for_update = true

  service_account {
    email  = google_service_account.bastion.email
    scopes = ["cloud-platform"]
  }

  /* local-exec providers may run before the host has fully initialized.
  However, they are run sequentially in the order they were defined.
  This provider is used to block the subsequent providers until the instance is available. */
  provisioner "local-exec" {
    command = <<EOF
        READY=""
        for i in $(seq 1 20); do
          if gcloud compute ssh ${var.bastion_name} --project ${var.project_id} --zone ${var.region}-a --command uptime; then
            READY="yes"
            break;
          fi
          echo "Waiting for ${var.bastion_name} to initialize..."
          sleep 10;
        done
        if [[ -z $READY ]]; then
          echo "${var.bastion_name} failed to start in time."
          echo "Please verify that the instance starts and then re-run `terraform apply`"
          exit 1
        fi
EOF
  }

  scheduling {
    preemptible       = true
    automatic_restart = false
  }
}
