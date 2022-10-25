#!/usr/bin/env bash

# admiral.sh - multiplexes helm

# USAGE: $0
# Commands:
#
# sync           | provisions all helmfile deployed resources on the cluster in each workspace
# wait_sync      | waits for teh above to complete before returning
# destroy        | destroys all helmfile deployed resources on the cluster in each workspace
# wait_destroy   | waits for the above to complete before returning

# DEPENDENCIES:
# - yq (https://github.com/mikefarah/yq)
# - helm (https://github.com/helm/helm)
# - helmfile (https://github.com/helmfile/helmfile)

# cd to the directory of the script
# leave this as the first command else all of the paths in this file will be incorrect
cd "$(dirname "$0")" || exit 1

###########################################################################################
# VARS
###########################################################################################

# can be overridden by passing environment variables
# eg NAME_PREFIX=foo LOG_DIR=/bar/baz ./admiral.sh some_command

# @description: the name with which to prefix all terraform resource names
# @default: "manual"
# @type: string
NAME_PREFIX="${NAME_PREFIX:-manual}"

# @description: the file from which to read workspace configuration
# @default: "../.runtime-${NAME_PREFIX}.yaml"
# @type: string
RUNTIME_DATA_FILE="${RUNTIME_DATA_FILE:-../.runtime-${NAME_PREFIX}.yaml}"

# @description: the directory in which to write write log files
# @default: "./logs"
# @type: string
LOG_DIR="${LOG_DIR:-./logs}"

###########################################################################################
# END VARS
###########################################################################################

###########################################################################################
# FUNCTIONS
###########################################################################################

### UX

# runs `helmfile sync` on each of the workspaces in the background and returns immediately
sync() {
  multiplex_helmfile sync
}

# runs `helmfile destroy` on each of the workspaces in the background and returns immediately
destroy() {
  multiplex_helmfile destroy
}

# waits for the sync on each workspace to complete and returns when they are all complete
wait_sync() {
  wait_command sync
}

# waits for the destroy on each workspace to complete and returns when they are all complete
wait_destroy() {
  wait_command destroy
}

# TODO make this functional
# outputs only the last workspace's stdout and stderr of the helmfile sync command and returns
# when the destroy command is complete
watch_sync() {

  return

#  last_workspace=$(yq '.workspaces[(.workspaces | length) - 1].name' "${RUNTIME_DATA_FILE}")
#  ( tail -f -n0 "${LOG_DIR}/${last_workspace}/helmfile_sync.log" & ) | grep -q "All helmfiles synced"
}

### LIBRARY

# wait for `$1 complete` (case insensitive) in the log files of the helmfile command on each of the
# workspaces and return when all files are populated
wait_command() {

  command="${1}"
  if [ -z "${command}" ]; then
    echo "[ERROR] ${0} called with no command"
    exit 1
  fi

  # becomes 0 when all workspaces are finished
  adder=1
  # while all workspaces are not finished
  while [ "${adder}" -gt 0 ]; do
    # lol sike
    adder=0
    # for each workspace in RUNTIME_DATA_FILE
    for workspace in $(yq -r '.workspaces[].name' "${RUNTIME_DATA_FILE}"); do
      # if we don't see command complete in the log
      if ! grep -q "Helmfile ${command} complete" "${LOG_DIR}/${workspace}/helmfile_${command}.log"; then
        # add one to the adder
        (( adder++ ))
      fi
    done
    # if any of the workspaces are not finished, pause 10 seconds
    if [ "${adder}" -gt 0 ]; then
      sleep 10
    fi
  done
  echo "[INFO] Helmfile ${command}s complete"
}

# executes the same helmfile command per defined workspace, returns immediately
multiplex_helmfile() {

  command="${1}"
  echo "[INFO] Init helmfile ${command}s"
  # loop through all the workspaces
  for workspace in $(yq -r '.workspaces[].name' "${RUNTIME_DATA_FILE}"); do

    # set variables
    port_number=$(yq -r '(.workspaces[] | select(.name == "'"${workspace}"'")).proxy.port' "${RUNTIME_DATA_FILE}")
    ctx=$(yq -r '(.workspaces[] | select(.name == "'"${workspace}"'")).kubectl.context' "${RUNTIME_DATA_FILE}")
    kube_contexts=$(kubectl config view | yq -r '.contexts[].name')
    cluster_name=$(yq -r '(.workspaces[] | select(.name == "'"${workspace}"'")).kubectl.cluster_name' "${RUNTIME_DATA_FILE}")
    registry_name=$(yq -r '(.workspaces[] | select(.name == "'"${workspace}"'")).registry_name' "${RUNTIME_DATA_FILE}")

    # ensure workspace log dir exists
    mkdir -p "${LOG_DIR}/${workspace}"
    # ensure expected context is inside of kubeconfig
    if [[ "${kube_contexts}" == *"${ctx}"* ]]; then
      # background a task that starts helmfile and notifies of completion in the workspace command log
      # shellcheck disable=SC2068
      ( (HTTPS_PROXY="localhost:${port_number}" CLUSTER_NAME="${cluster_name}" REGISTRY_NAME="${registry_name}" helmfile ${@} --kube-context "${ctx}" &> "${LOG_DIR}/${workspace}/helmfile_${command}.log") \
        && (echo "Helmfile ${command} complete!" > "${LOG_DIR}/${workspace}/helmfile_${command}.log") ) &
    else
      echo "[ERROR] Couldn't find ${ctx} in kubectl config" | tee "${LOG_DIR}/${workspace}/helmfile_${command}.log"
      exit 1
    fi
  done
}

# pre-flight checks run before anything called via the command line, eg `./admiral.sh command`
check_dependencies() {

  # check yq
  if ! command -v yq &> /dev/null; then
    echo "[ERROR] yq is not installed"
    exit 1
  fi

  # check helm
  if ! command -v helm &> /dev/null; then
    echo "[ERROR] helm is not installed"
    exit 1
  fi

  # make log dir
  mkdir -p "${LOG_DIR}"

  # remove old logs
  for workspace in $(yq -r '.workspaces[].name' "${RUNTIME_DATA_FILE}"); do
    rm -f "${LOG_DIR:?}/${workspace}/*"
  done

  # check for existence of RUNTIME_DATA_FILE
  if [ ! -f "${RUNTIME_DATA_FILE}" ]; then
    echo "[ERROR] RUNTIME_DATA_FILE ${RUNTIME_DATA_FILE} not found"
    exit 1
  fi

  # validate/lint RUNTIME_DATA_FILE
  # this just checks to ensure that the file contains a valid array as the root element
  if ! (yq --exit-status 'tag == "!!map" or tag == "!!seq"' "${RUNTIME_DATA_FILE}" &> /dev/null); then
    echo "[ERROR] RUNTIME_DATA_FILE ${RUNTIME_DATA_FILE} could not be linted"
  fi
}

###########################################################################################
# END FUNCTIONS
###########################################################################################

###########################################################################################
# MAIN
###########################################################################################

# if check dependencies returns anything but 0, error out
if check_dependencies; then
  if [ $# -eq 0 ]; then
    # if no arguments, run the default command, sync
    sync
  else
    # run $1 as the name of the function above and pass any arguments
    "${@}"
  fi
fi

###########################################################################################
# END MAIN
###########################################################################################
