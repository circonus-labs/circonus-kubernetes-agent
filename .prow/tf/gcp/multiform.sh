#!/usr/bin/env bash

# multiform.sh - multiplexes terraform

# USAGE: $0 [COMMAND]
# Commands:
#
# destroy        | destroys all resources in all workspaces
# plan           | plans all resources in all workspaces
# apply          | provisions all resources in all workspaces
# kubeconfig     | sets up kubeconfig with contexts for all clusters
# proxy          | sets up proxy for each cluster using gcloud and ssh
# teardown_proxy | kills all running proxy processes

# DEPENDENCIES:
# - yq ()
# - terraform ()
# - gcloud ()

# cd to the directory of the script
# leave this as the first command else all of the paths in this file will be incorrect
cd "$(dirname "$0")" || exit

###########################################################################################
# VARS
###########################################################################################

# can be overridden by passing environment variables
# eg NAME_PREFIX=foo LOG_DIR=/bar/baz ./multiform.sh some_command

# @description: the name with which to prefix all terraform resource names
# @default: "manual"
# @type: string
NAME_PREFIX="${NAME_PREFIX:-manual}"

# @description: the file from which to read workspace configuration
# @default: "../../.runtime-${NAME_PREFIX}.yaml"
# @type: string
RUNTIME_DATA_FILE="${RUNTIME_DATA_FILE:-../../.runtime-${NAME_PREFIX}.yaml}"

# @description: the directory in which to write write log files
# @default: "./logs"
# @type: string
LOG_DIR="${LOG_DIR:-./logs}"

# @description: the flags to add to terraform commands to disable user confirmation/input
# @default: "-input=false -auto-approve"
# @type: string
NOINPUT_FLAGS="${NOINPUT_FLAGS:--input=false -auto-approve}"

# @description: the directory to which to write sockets for proxy ssh connection control
# @default: "./.multiform-proxy/sockets"
# @type: string
PROXY_SOCK_DIR="${PROXY_SOCK_DIR:-./.multiform-proxy/sockets}"

###########################################################################################
# END VARS
###########################################################################################

###########################################################################################
# FUNCTIONS
###########################################################################################

### UX

# runs `terraform plan on each of the workspaces` in the background and returns immediately
plan() {
  for_workspace_vars_terraform plan
}

# runs `terraform apply` on each of the workspaces
apply() {

  # shellcheck disable=SC2086
  for_workspace_vars_terraform apply ${NOINPUT_FLAGS}
}

# runs `terraform destroy` on each of the workspaces in the background and returns immediately
destroy() {

  # we want this to expand
  # shellcheck disable=SC2086
  for_workspace_vars_terraform destroy ${NOINPUT_FLAGS}
}

# waits for the apply on each workspaces to complete and returns when they are all complete
wait_apply() {
  wait_command apply
}

# waits for the destroy on each workspaces to complete and returns when they are all complete
wait_destroy() {
  wait_command destroy
}

# sets up kubeconfig for each workspace
kubeconfig() {

  echo "[INFO] Init fetching kubeconfig contexts"
  # loop through the workspaces
  for workspace in $(yq -r '.workspaces[].name' "${RUNTIME_DATA_FILE}"); do

    # set up variables
    project_id=$(TF_WORKSPACE="${workspace}" terraform output --raw project_id)
    region=$(TF_WORKSPACE="${workspace}" terraform output --raw region)
    cluster_name=$(TF_WORKSPACE="${workspace}" terraform output --raw cluster_name)

    # the command to run to do this comes from terraform output
    cred_cmd=$(TF_WORKSPACE="${workspace}" terraform output --raw get_credentials)
    # execute the command from the terraform output
    eval "${cred_cmd} &> ${LOG_DIR}/${workspace}/gcloud_kubeconfig.log"

    # update RUNTIME_DATA_FILE with this cluster's kubecontext
    yq -i '(.workspaces[] | select(.name == "'"${workspace}"'")).kubectl.context = "gke_'"${project_id}"'_'"${region}"'_'"${cluster_name}"'"' "${RUNTIME_DATA_FILE}"
    # update RUNTIME_DATA_FILE with this cluster's name
    yq -i '(.workspaces[] | select(.name == "'"${workspace}"'")).kubectl.cluster_name = "'"${cluster_name}"'"' "${RUNTIME_DATA_FILE}"
  done
  echo "[INFO] Kubeconfig contexts generated successfully"
}

# zzz
registry() {

  echo "[INFO] Init fetching registries..."
  # loop through the workspaces
  for workspace in $(yq -r '.workspaces[].name' "${RUNTIME_DATA_FILE}"); do

    # set up variables
    project_id=$(TF_WORKSPACE="${workspace}" terraform output --raw project_id)
    region=$(TF_WORKSPACE="${workspace}" terraform output --raw region)
    registry_name=$(TF_WORKSPACE="${workspace}" terraform output --raw registry_name)

    # update RUNTIME_DATA_FILE with registry name
    yq -i '(.workspaces[] | select(.name == "'"${workspace}"'")).registry_name = "'"${region}"'-docker.pkg.dev/'"${project_id}"'/'"${registry_name}"'"' "${RUNTIME_DATA_FILE}"
  (cd ../../../ && make build REGISTRY_NAME="${region}-docker.pkg.dev/${project_id}/${registry_name}")
  done
  echo "[INFO] Registry images pushed successfully"
}

# set up a reverse ssh tunnel per workspace
# ssh tunnel -> bastion -> gke control plane
proxy() {

  # if the proxy pid file exists, then tearing down the proxy must have not finished.
  if [ -d "${PROXY_SOCK_DIR}" ]; then
    # tear it down again
    teardown_proxy
  fi

  # ensure PROXY_SOCK_DIR exists
  mkdir -p "${PROXY_SOCK_DIR}"

  echo "[INFO] Init starting proxies"

  # loop through the workspaces
  for workspace in $(yq -r '.workspaces[].name' "${RUNTIME_DATA_FILE}"); do

    # set up variables
    port_number=$(yq -r '.workspaces[] | select(.name == "'"${workspace}"'") | .proxy.port' "${RUNTIME_DATA_FILE}")
    bastion_name=$(TF_WORKSPACE="${workspace}" terraform output --raw bastion_name)
    project_id=$(TF_WORKSPACE="${workspace}" terraform output --raw project_id)
    zone=$(TF_WORKSPACE="${workspace}" terraform output --raw bastion_zone)

    # the actual command string to run to start the proxy
    PROXY_CMD="gcloud compute ssh ${bastion_name} --project ${project_id} --zone ${zone} -- -L${port_number}:127.0.0.1:8888 -S ${PROXY_SOCK_DIR}/${workspace}.sock -M -f tail -f /dev/null"
    # execute proxy init sequence
    eval "${PROXY_CMD} &> ${LOG_DIR}/${workspace}/gcloud_proxy.log"
  done
  echo "[INFO] Proxies started successfully"
}

# send each SSH tunnel instance the exit command via its respective ControlMaster socket
teardown_proxy() {

  # if the PROXY_SOCK_DIR doesn't exist, we have nothing to do
  if ! [ -d "${PROXY_SOCK_DIR}" ]; then
    return
  fi

  echo "[INFO] Init closing SSH tunnels"

  # while PROXY_SOCK_DIR is not empty
  while [ -n "$(command ls -A "${PROXY_SOCK_DIR}")" ]; do
    # loop through the workspaces
    for workspace in $(yq -r '.workspaces[].name' "${RUNTIME_DATA_FILE}"); do
      # if the proxy ControlMaster socket doesn't exist, the proxy for that workspace is already cleaned up, and we can skip this run
      if [ ! -S "${PROXY_SOCK_DIR}/${workspace}.sock" ]; then
        continue
      fi
      # set up variables
      port_number=$(yq -r '.workspaces[] | select(.name == "'"${workspace}"'") | .proxy.port' "${RUNTIME_DATA_FILE}")
      bastion_name=$(TF_WORKSPACE="${workspace}" terraform output --raw bastion_name)
      project_id=$(TF_WORKSPACE="${workspace}" terraform output --raw project_id)
      zone=$(TF_WORKSPACE="${workspace}" terraform output --raw bastion_zone)
      # the actual command string to run to kill the proxy
      PROXY_CMD="gcloud compute ssh ${bastion_name} --project ${project_id} --zone ${zone} -- -S ${PROXY_SOCK_DIR}/${workspace}.sock -O exit"
      # execute proxy exit sequence
      eval "${PROXY_CMD} &> ${LOG_DIR}/${workspace}/gcloud_teardown_proxy.log"
    done
    # if everything hasn't been cleaned up, wait 5 before retrying
    if [ -n "$(command ls -A "${PROXY_SOCK_DIR}")" ]; then sleep 5; fi
  done
  # this isn't needed any longer
  rm -rf "${PROXY_SOCK_DIR}"
  echo "[INFO] SSH tunnels closed successfully"
}

# TODO make this functional
# outputs the stdout and stderr of the terraform apply on the last workspace and returns
# when the destroy command is complete
watch_apply() {

  return

  # last_workspace=$(yq '.workspaces[(.workspaces | length) - 1].name' "${RUNTIME_DATA_FILE}")
  # ( tail -f -n0 "${LOG_DIR}/${last_workspace}/apply.log" & ) | grep -q "Apply complete!"
}

# TODO make this functional
# outputs the stdout and stderr of the terraform destroy command on the last workspace and returns
# when the destroy command is complete
watch_destroy() {

  return

  # last_workspace=$(yq '.workspaces[(.workspaces | length) - 1].name' "${RUNTIME_DATA_FILE}")
  # ( tail -f -n0 "${LOG_DIR}/${last_workspace}/destroy.log" & ) | grep -q "Destroy complete"
}

### LIBRARY

# wait for `$1 complete` (case insensitive) in the log files of the terraform command on each of the
# workspaces and return when all files are populated
wait_command() {

  command="${1}"
  if [ -z "${command}" ]; then
    echo "[ERROR] ${0} called with no argument" | tee "${LOG_DIR}/${workspace}/${command}.log"
    exit 1
  fi

  # becomes 0 when all workspaces are finished
  adder=1
  # while all workspaces are not finished
  while [ "${adder}" -gt 0 ]; do
    # lol sike
    adder=0
    # for each workspace
    for workspace in $(yq -r '.workspaces[].name' "${RUNTIME_DATA_FILE}"); do
      # if we don't see command complete in the log
      if ! grep -iq "${command} complete" "${LOG_DIR}/${workspace}/${command}.log"; then
        # add one to the adder
        (( adder++ ))
      fi
    done
    # if one of the workspaces is not finished, pause 10 seconds
    if [ "${adder}" -gt 0 ]; then
      sleep 10
    fi
  done
  echo "[INFO] Terraform ${command}s complete"
}

# executes the same terraform command per defined workspace, returns immediately
for_workspace_vars_terraform() {

  command="${1}"
  echo "[INFO] Init Terraform ${command}s"
  # loop through workspaces
  for workspace in $(yq -r '.workspaces[].name' "${RUNTIME_DATA_FILE}"); do
    # build variables to pass to terraform
    workspace_vars=""
    # for each one of the variables in the workspace's vars field
    for var in $(yq -r '.workspaces[] | select(.name == "'"${workspace}"'") | .vars[].name' "${RUNTIME_DATA_FILE}"); do
      # add the k,v to the workspace_vars builder
      workspace_vars="${workspace_vars} -var ${var}=$(yq -r '.workspaces[] | select(.name == "'"${workspace}"'") | .vars[] | select(.name == "'"${var}"'") | .value' "${RUNTIME_DATA_FILE}")"
    done

    # ensure workspace log dir exists
    mkdir -p "${LOG_DIR}/${workspace}"
    # execute terraform command with vars as arguments
    # shellcheck disable=SC2068,SC2086
    TF_WORKSPACE="${workspace}" terraform ${@} ${workspace_vars} &>"${LOG_DIR}/${workspace}/${command}.log" &
  done
}

# pre-flight checks run before anything called via the command line, eg `./multiform.sh command`
check_dependencies() {

  # make log dir
  mkdir -p "${LOG_DIR}"

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

  # check for existence of RUNTIME_DATA_FILE
  if [ ! -f "${RUNTIME_DATA_FILE}" ]; then
    echo "[ERROR] RUNTIME_DATA_FILE ${RUNTIME_DATA_FILE} not found"
    exit 1
  fi

  # validate/lint RUNTIME_DATA_FILE
  # this just checks to ensure that the file contains a valid array as the root element
  if ! (yq --exit-status 'tag == "!!map" or tag == "!!seq"' "${RUNTIME_DATA_FILE}" &> /dev/null); then
    echo "[ERROR] RUNTIME_DATA_FILE ${RUNTIME_DATA_FILE} could not be linted"
    exit 1
  fi

  # check terraform and workspaces
  if [ ! -f .terraform.lock.hcl ]; then
    terraform init
  fi
  for workspace in $(yq -r '.workspaces[].name' "${RUNTIME_DATA_FILE}"); do
    if [ ! -d "./terraform.tfstate.d/${workspace}" ]; then
      terraform workspace new "${workspace}"
    fi
  done
}

###########################################################################################
# END FUNCTIONS
###########################################################################################

###########################################################################################
# MAIN
###########################################################################################

# if check dependencies returns anything but 0, error out
if check_dependencies; then
  # run $1 as the name of the function above and pass any arguments
  "${@}"
else
  echo "[ERROR] dependency check failed" | tee "${LOG_DIR}/multiform.log"
fi

###########################################################################################
# END MAIN
###########################################################################################
