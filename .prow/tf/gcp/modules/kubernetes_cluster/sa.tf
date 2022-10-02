/**
 * Copyright 2022 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// This file was automatically generated from a template in ./autogen/main

locals {
  service_account        = google_service_account.cluster_service_account.email
  registry_projects_list = [var.project_id]
}

resource "random_string" "cluster_service_account_suffix" {
  upper   = false
  lower   = true
  special = false
  length  = 4
}

resource "google_service_account" "cluster_service_account" {
  project      = var.project_id
  account_id   = "${substr(var.cluster_name, 0, min(15, length(var.cluster_name)))}-${random_string.cluster_service_account_suffix.result}"
  display_name = "Terraform-managed service account for cluster ${var.cluster_name}"
}

resource "google_project_iam_member" "cluster_service_account-log_writer" {
  project = google_service_account.cluster_service_account.project
  role    = "roles/logging.logWriter"
  member  = "serviceAccount:${google_service_account.cluster_service_account.email}"
}

resource "google_project_iam_member" "cluster_service_account-metric_writer" {
  project = google_project_iam_member.cluster_service_account-log_writer.project
  role    = "roles/monitoring.metricWriter"
  member  = "serviceAccount:${google_service_account.cluster_service_account.email}"
}

resource "google_project_iam_member" "cluster_service_account-monitoring_viewer" {
  project = google_project_iam_member.cluster_service_account-metric_writer.project
  role    = "roles/monitoring.viewer"
  member  = "serviceAccount:${google_service_account.cluster_service_account.email}"
}

resource "google_project_iam_member" "cluster_service_account-resourceMetadata-writer" {
  project = google_project_iam_member.cluster_service_account-monitoring_viewer.project
  role    = "roles/stackdriver.resourceMetadata.writer"
  member  = "serviceAccount:${google_service_account.cluster_service_account.email}"
}

resource "google_project_iam_member" "cluster_service_account-gcr" {
  for_each = toset(local.registry_projects_list)
  project  = each.key
  role     = "roles/storage.objectViewer"
  member   = "serviceAccount:${google_service_account.cluster_service_account.email}"
}

resource "google_project_iam_member" "cluster_service_account-artifact-registry" {
  for_each = toset(local.registry_projects_list)
  project  = each.key
  role     = "roles/artifactregistry.reader"
  member   = "serviceAccount:${google_service_account.cluster_service_account.email}"
}
