#!/usr/bin/env bash

# multiform.sh - multiplexes terraform

# USAGE: $0 [COMMAND]
# Commands:
#
# destroy        | destroys all gcp resources in all workspaces
# plan           | plans all gcp resources in all workspaces
# apply          | provisions all gcp resources in all workspaces
# kubeconfig     | sets up kubeconfig with contexts for all clusters
# proxy          | sets up proxy for each cluster using gcloud and ssh
# teardown_proxy | kills all running proxy processes

# DEPENDENCIES:
# - yq ()
# - terraform ()
# - gcloud ()

cd "$(dirname "$0")" || exit

NAME_PREFIX="${NAME_PREFIX:=manual}"
RUNTIME_DATA_FILE="../../.runtime-${NAME_PREFIX}.yaml"
LOG_DIR="./logs"
FLAGS="-input=false -auto-approve"
PROXY_SOCK_DIR="./.multiform-proxy/sockets"

destroy() {

  # shellcheck disable=SC2086
  for_workspace_vars_terraform destroy ${FLAGS}
}

wait_destroy() {

  finished=false
  while [ "${finished}" != "true" ]; do
    adder=0
    for workspace in $(yq -r '.workspaces[].name' "${RUNTIME_DATA_FILE}"); do
      if grep -q "Destroy complete" "${LOG_DIR}/${workspace}/destroy.log"; then
        :
      else
        (( adder++ ))
      fi
    done
    if [ "${adder}" -gt 0 ]; then
      sleep 10
    else
      echo "All destroys complete"
      finished="true"
    fi
  done
}

watch_destroy() {

  last_workspace=$(yq '.workspaces[(.workspaces | length) - 1].name' "${RUNTIME_DATA_FILE}")
	( tail -f -n0 "${LOG_DIR}/${last_workspace}/destroy.log" & ) | grep -q "Destroy complete"
}

plan() {

  for_workspace_vars_terraform plan
}

apply() {

  # shellcheck disable=SC2086
  for_workspace_vars_terraform apply ${FLAGS}
}

wait_apply() {

  finished=false
  while [ "${finished}" != "true" ]; do
    adder=0
    for workspace in $(yq -r '.workspaces[].name' "${RUNTIME_DATA_FILE}"); do
      if grep -q "Apply complete" "${LOG_DIR}/${workspace}/apply.log"; then
        :
      else
        (( adder++ ))
      fi
    done
    if [ "${adder}" -gt 0 ]; then
      sleep 10
    else
      echo "All applys complete"
      finished="true"
    fi
  done
}

watch_apply() {

  last_workspace=$(yq '.workspaces[(.workspaces | length) - 1].name' "${RUNTIME_DATA_FILE}")
	( tail -f -n0 "${LOG_DIR}/${last_workspace}/apply.log" & ) | grep -q "Apply complete!"
}

for_workspace_vars_terraform() {

  for workspace in $(yq -r '.workspaces[].name' "${RUNTIME_DATA_FILE}"); do
    workspace_vars=""
    for var in $(yq -r '.workspaces[] | select(.name == "'"${workspace}"'") | .vars[].name' "${RUNTIME_DATA_FILE}"); do
      workspace_vars="${workspace_vars} -var ${var}=$(yq -r '.workspaces[] | select(.name == "'"${workspace}"'") | .vars[] | select(.name == "'"${var}"'") | .value' ${RUNTIME_DATA_FILE})"
    done

    mkdir -p "${LOG_DIR}/${workspace}"
    # shellcheck disable=SC2068,SC2086
    TF_WORKSPACE="${workspace}" terraform ${@} ${workspace_vars} &>"${LOG_DIR}/${workspace}/${1}.log" &
  done
}

kubeconfig() {

  for workspace in $(yq -r '.workspaces[].name' "${RUNTIME_DATA_FILE}"); do
    cred_cmd=$(TF_WORKSPACE="${workspace}" terraform output --raw get_credentials)
    project_id=$(TF_WORKSPACE="${workspace}" terraform output --raw project_id)
    region=$(TF_WORKSPACE="${workspace}" terraform output --raw region)
    cluster_name=$(TF_WORKSPACE="${workspace}" terraform output --raw cluster_name)

    eval "${cred_cmd}"
    yq -i '(.workspaces[] | select(.name == "'"${workspace}"'")).kubectl.context = "gke_'"${project_id}"'_'"${region}"'_'"${cluster_name}"'"' "${RUNTIME_DATA_FILE}"
    yq -i '(.workspaces[] | select(.name == "'"${workspace}"'")).kubectl.cluster_name = "'"${cluster_name}"'"' "${RUNTIME_DATA_FILE}"
  done
}

proxy() {

  # if the proxy pid file exists, then tearing down the proxy must have not finished.
  if [ -d "${PROXY_SOCK_DIR}" ]; then
    teardown_proxy
  fi

  mkdir -p "${PROXY_SOCK_DIR}"
  for workspace in $(yq -r '.workspaces[].name' "${RUNTIME_DATA_FILE}"); do
    port_number=$(yq -r '.workspaces[] | select(.name == "'"${workspace}"'") | .proxy.port' "${RUNTIME_DATA_FILE}")
    bastion_name=$(TF_WORKSPACE="${workspace}" terraform output --raw bastion_name)
    project_id=$(TF_WORKSPACE="${workspace}" terraform output --raw project_id)
    zone=$(TF_WORKSPACE="${workspace}" terraform output --raw bastion_zone)
    PROXY_CMD="gcloud compute ssh ${bastion_name} --project ${project_id} --zone ${zone} -- -L${port_number}:127.0.0.1:8888 -S ${PROXY_SOCK_DIR}/${workspace}.sock -M -f tail -f /dev/null"
    eval "${PROXY_CMD}"
  done
}

teardown_proxy() {

  while [ -n "$(command ls -A ${PROXY_SOCK_DIR})" ]; do
    for workspace in $(yq -r '.workspaces[].name' "${RUNTIME_DATA_FILE}"); do
      port_number=$(yq -r '.workspaces[] | select(.name == "'"${workspace}"'") | .proxy.port' "${RUNTIME_DATA_FILE}")
      bastion_name=$(TF_WORKSPACE="${workspace}" terraform output --raw bastion_name)
      project_id=$(TF_WORKSPACE="${workspace}" terraform output --raw project_id)
      zone=$(TF_WORKSPACE="${workspace}" terraform output --raw bastion_zone)
      PROXY_CMD="gcloud compute ssh ${bastion_name} --project ${project_id} --zone ${zone} -- -S ${PROXY_SOCK_DIR}/${workspace}.sock -O exit"
      eval "${PROXY_CMD}"
    done
    if [ -n "$(command ls -A ${PROXY_SOCK_DIR})" ]; then sleep 5; fi
  done
  rm -rf "${PROXY_SOCK_DIR}"
}

check_dependencies() {

  # check yq
  if ! command -v yq &> /dev/null; then
    echo "[ERROR] yq is not installed"
    exit 1
  fi

  # check terraform
  if ! command -v terraform &> /dev/null; then
    echo "[ERROR] terraform is not installed"
    exit 1
  fi

  # check gcloud
  if ! command -v gcloud &> /dev/null; then
    echo "[ERROR] Google Cloud SDK is not installed"
    exit 1
  fi

  if [ ! -f "${RUNTIME_DATA_FILE}" ]; then
    echo "[ERROR] RUNTIME_DATA_FILE ${RUNTIME_DATA_FILE} not found"
    exit 1
  fi

  if [ ! -f .terraform.lock.hcl ]; then
    terraform init
    for workspace in $(yq -r '.workspaces[].name' "${RUNTIME_DATA_FILE}"); do
      terraform workspace new "${workspace}"
    done
  fi

  # make log dir
  mkdir -p "${LOG_DIR}"
}

if check_dependencies; then
  "${@}"
fi
