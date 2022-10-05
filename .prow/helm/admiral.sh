#!/usr/bin/env bash

# admiral.sh - multiplexes helm

# USAGE: $0
# Commands:

# DEPENDENCIES:
# - yq ()
# - helm ()
# - helmfile ()

cd "$(dirname "$0")" || exit 1

NAME_PREFIX="${NAME_PREFIX:=manual}"
LOG_DIR="./logs/"
RUNTIME_DATA_FILE="../.runtime-${NAME_PREFIX}.yaml"

wait_sync() {

  finished="false"
  while [ "${finished}" != "true" ]; do
    for workspace in $(yq -r '.workspaces[].name' "${RUNTIME_DATA_FILE}"); do
      if grep -q "All helmfiles synced" "${LOG_DIR}/${workspace}/helmfile_sync.log"; then
        finished="true"
        break
      fi
    done
    if [ "${finished}" != "true" ]; then
      sleep 10
    fi
  done
  echo "All applys complete"
}

# TODO make this functional
watch_sync() {

  last_workspace=$(yq '.workspaces[(.workspaces | length) - 1].name' "${RUNTIME_DATA_FILE}")
	( tail -f -n0 "${LOG_DIR}/${last_workspace}/helmfile_sync.log" & ) | grep -q "All helmfiles synced"
}

sync() {

  multiplex_helmfile sync
  last_workspace=$(yq '.workspaces[(.workspaces | length) - 1].name' "${RUNTIME_DATA_FILE}")
  echo "All helmfiles synced" >> "${LOG_DIR}/${last_workspace}/helmfile_sync.log"
}

destroy() {

  multiplex_helmfile destroy
}

multiplex_helmfile() {

  for workspace in $(yq -r '.workspaces[].name' "${RUNTIME_DATA_FILE}"); do
    port_number=$(yq -r '(.workspaces[] | select(.name == "'"${workspace}"'")).proxy.port' "${RUNTIME_DATA_FILE}")
    ctx=$(yq -r '(.workspaces[] | select(.name == "'"${workspace}"'")).kubectl.context' "${RUNTIME_DATA_FILE}")
    kube_contexts=$(kubectl config view | yq -r '.contexts[].name')
    cluster_name=$(yq -r '(.workspaces[] | select(.name == "'"${workspace}"'")).kubectl.cluster_name' "${RUNTIME_DATA_FILE}")

    mkdir -p "${LOG_DIR}/${workspace}"
    if [[ "${kube_contexts}" == *"${ctx}"* ]]; then
      HTTPS_PROXY="localhost:${port_number}" CLUSTER_NAME="${cluster_name}" helmfile "${1}" --kube-context "${ctx}" &> "${LOG_DIR}/${workspace}/helmfile_${1}.log" &
    else
      echo "[ERROR] Couldn't find ${ctx} in kubectl config"
      exit 1
    fi
  done
}

check_dependencies() {

  if [ ! -f "${RUNTIME_DATA_FILE}" ]; then
    echo "[ERROR] RUNTIME_DATA_FILE ${RUNTIME_DATA_FILE} not found"
    exit 1
  fi

  # TODO validate RUNTIME_DATA_FILE

  # make log dir
  mkdir -p "${LOG_DIR}"

  # remove old logs
  for workspace in $(yq -r '.workspaces[].name' "${RUNTIME_DATA_FILE}"); do
    rm -rf "${LOG_DIR:?}/${workspace}"
  done

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

}

if check_dependencies; then
  if [ $# -eq 0 ]; then
    sync
  else
    "${@}"
  fi
fi
