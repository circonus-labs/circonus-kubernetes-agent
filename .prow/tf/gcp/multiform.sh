#!/usr/bin/env bash

# multiform.sh - multiplexes terraform

# USAGE: $0 [COMMAND]
# Commands:
#
# destroy  | destroys all resources in all workspaces
# plan     | plans all resources in all workspaces
# apply    | provisions all resources in all workspaces

# DEPENDENCIES:
# - yq ()

cd "$(dirname "$0")" || exit

DATA_FILE=./multiform.yaml
LOG_DIR=./logs/
FLAGS="-input=false -auto-approve"

destroy() {
  # shellcheck disable=SC2086
  for_workspace_vars_terraform destroy ${FLAGS}

#  for workspace in $(yq -r '.workspaces[].name' "${DATA_FILE}"); do
#    workspace_vars=""
#    for var in $(yq -r '.workspaces[] | select(.name == '"${workspace}"') | .vars[].name' "${DATA_FILE}"); do
#      workspace_vars="${workspace_vars} -var ${var}=$(yq -r '.workspaces[] | select(.name == '"${workspace}"') | .vars[] | select(.name == "kubernetes_version") | .value' ${DATA_FILE})"
#    done
#
#    mkdir -p "${LOG_DIR}/${workspace}"
#    # shellcheck disable=SC2086
#    TF_WORKSPACE="${workspace}" terraform destroy ${FLAGS} ${workspace_vars} &>"${LOG_DIR}/${workspace}/destroy.log" &
#  done
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

#  for workspace in $(yq -r '.workspaces[].name' "${DATA_FILE}"); do
#    workspace_vars=""
#    for var in $(yq -r '.workspaces[] | select(.name == '"${workspace}"') | .vars[].name' "${DATA_FILE}"); do
#      workspace_vars="${workspace_vars} -var ${var}=$(yq -r '.workspaces[] | select(.name == '"${workspace}"') | .vars[] | select(.name == "kubernetes_version") | .value' ${DATA_FILE})"
#    done
#
#    mkdir -p "${LOG_DIR}/${workspace}"
#    # shellcheck disable=SC2086
#    TF_WORKSPACE="${workspace}" terraform plan ${workspace_vars} &>"${LOG_DIR}/${workspace}/plan.log" &
#  done
}

apply() {
  # shellcheck disable=SC2086
  for_workspace_vars_terraform apply ${FLAGS}

#  for workspace in $(yq -r '.workspaces[].name' "${DATA_FILE}"); do
#    workspace_vars=""
#    for var in $(yq -r '.workspaces[] | select(.name == '"${workspace}"') | .vars[].name' "${DATA_FILE}"); do
#      workspace_vars="${workspace_vars} -var ${var}=$(yq -r '.workspaces[] | select(.name == '"${workspace}"') | .vars[] | select(.name == "kubernetes_version") | .value' ${DATA_FILE})"
#    done
#
#    mkdir -p "${LOG_DIR}/${workspace}"
#    # shellcheck disable=SC2086
#    TF_WORKSPACE="${workspace}" terraform apply ${FLAGS} ${workspace_vars} &>"${LOG_DIR}/${workspace}/apply.log" &
#  done
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
      workspace_vars="${workspace_vars} -var ${var}=$(yq -r '.workspaces[] | select(.name == '"${workspace}"') | .vars[] | select(.name == "kubernetes_version") | .value' ${DATA_FILE})"
    done

    mkdir -p "${LOG_DIR}/${workspace}"
    # shellcheck disable=SC2068,SC2086
    TF_WORKSPACE="${workspace}" terraform ${@} ${workspace_vars} &>"${LOG_DIR}/${workspace}/${1}.log" &
  done
}

check_dependencies() {

  # make log dir
  mkdir -p "${LOG_DIR}"

  if [ ! -f .terraform.lock.hcl ]; then
    terraform init
    for workspace in $(yq -r '.workspaces[].name' "${DATA_FILE}"); do
      terraform workspace new "${workspace}"
    done
  fi

  # check yq
  if ! command -v yq &> /dev/null; then
    echo "[ERROR] yq is not installed"
    exit 1
  fi
}

if check_dependencies; then
  "${@}"
fi
