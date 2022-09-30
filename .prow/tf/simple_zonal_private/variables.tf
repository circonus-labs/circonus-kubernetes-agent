/**
 * Copyright 2018 Google LLC
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

variable "kubernetes_version" {
  description = "Kubernetes version"
}

variable "name_prefix" {
  description = "A suffix to append to resource names"
  default     = "manual"
}

variable "project_id" {
  description = "The project ID to host the cluster in"
  default     = "circonus-dev"
}

variable "region" {
  description = "The region to host the cluster in"
  default     = "us-central1"
}

variable "zones" {
  type        = list(string)
  description = "The zone to host the cluster in (required if is a zonal cluster)"
  default     = ["us-central1-a"]
}

