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

DATA_FILE=../../supported-k8s-versions.yaml
LOG_DIR=./logs/
FLAGS="-input=false -auto-approve"
PROXY_PID_FILE=".gcp-proxy.pid"

destroy() {
  # shellcheck disable=SC2086
  for_workspace_vars_terraform destroy ${FLAGS}
}

watch_destroy() {
  finished=false
  while [ "${finished}" != "true" ]; do
    adder=0
    for workspace in $(yq -r '.workspaces[].name' "${DATA_FILE}"); do
      if grep -q "Destroy complete" "${LOG_DIR}/${workspace}/destroy.log"; then
        :
      else
        echo "Still waiting on workspace: ${workspace}"
        (( adder++ ))
      fi
    done
    if [ "${adder}" -gt 0 ]; then
      echo "Destroy not complete, continuing to wait..."
      sleep 10
    else
      echo "Destroy complete"
      finished="true"
    fi
  done
}

plan() {
  for_workspace_vars_terraform plan
}

apply() {
  # shellcheck disable=SC2086
  for_workspace_vars_terraform apply ${FLAGS}
}

watch_apply() {
  finished=false
  while [ "${finished}" != "true" ]; do
    adder=0
    for workspace in $(yq -r '.workspaces[].name' "${DATA_FILE}"); do
      if grep -q "Apply complete" "${LOG_DIR}/${workspace}/apply.log"; then
        :
      else
        echo "Still waiting on workspace: ${workspace}"
        (( adder++ ))
      fi
    done
    if [ "${adder}" -gt 0 ]; then
      echo "Apply not complete, continuing to wait..."
      sleep 10
    else
      echo "Apply complete"
      finished="true"
    fi
  done
}

for_workspace_vars_terraform() {
  for workspace in $(yq -r '.workspaces[].name' "${DATA_FILE}"); do
    workspace_vars=""
    for var in $(yq -r '.workspaces[] | select(.name == '"${workspace}"') | .vars[].name' "${DATA_FILE}"); do
      workspace_vars="${workspace_vars} -var ${var}=$(yq -r '.workspaces[] | select(.name == '"${workspace}"') | .vars[] | select(.name == '"${var}"') | .value' ${DATA_FILE})"
    done

    mkdir -p "${LOG_DIR}/${workspace}"
    # shellcheck disable=SC2068,SC2086
    TF_WORKSPACE="${workspace}" terraform ${@} ${workspace_vars} &>"${LOG_DIR}/${workspace}/${1}.log" &
  done
}

kubeconfig() {

  for workspace in $(yq -r '.workspaces[].name' "${DATA_FILE}"); do
    eval TF_WORKSPACE="${workspace}" terraform output get_credentials
  done
}

proxy() {

  # if the proxy pid file exists, then tearing down the proxy must have not finished.
  if [ -f "${PROXY_PID_FILE}" ]; then
    teardown_proxy
  fi

  touch "${PROXY_PID_FILE}"
  for workspace in $(yq -r '.workspaces[].name' "${DATA_FILE}"); do
    PROXY_CMD=$(TF_WORKSPACE="${workspace}" terraform output bastion_open_tunnel_command)
    PROXY_CMD="${PROXY_CMD/L8888/L8$workspace}"
    # shellcheck disable=SC2086
    eval ${PROXY_CMD}
    echo "${!}" >> "${PROXY_PID_FILE}"
  done
}

teardown_proxy() {

  for pid in ${PROXY_PID_FILE}; do
    killed=false
    while [ "${killed}" != true ]; do
      kill -9 "${pid}"
      if ps -p ${pid} > /dev/null; then
        echo "Waiting for ${pid} to die..."
      else
        killed=true
      fi
      sleep 2
    done &
  done
  rm -f "${PROXY_PID_FILE}"
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

  if [ ! -f .terraform.lock.hcl ]; then
    terraform init
    for workspace in $(yq -r '.workspaces[].name' "${DATA_FILE}"); do
      terraform workspace new "${workspace}"
    done
  fi

  # make log dir
  mkdir -p "${LOG_DIR}"
}

if check_dependencies; then
  "${@}"
fi
