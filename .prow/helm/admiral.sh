#!/usr/bin/env bash

# admiral.sh - multiplexes helm

# USAGE: $0
# Commands:

# DEPENDENCIES:
# - yq ()
# - helm ()
# - helmfile ()

cd "$(dirname "$0")" || exit 1

LOG_DIR=./logs/

CONTEXTS=$(kubectl config view | yq -r '.contexts[].name')

main() {

  for ctx in ${CONTEXTS}; do
    mkdir -p "${LOG_DIR}/${ctx}"
    helmfile sync --kube-context "${ctx}" &> "${LOG_DIR}/${ctx}/helmfile_sync.log" &
  done

}

check_dependencies() {

  # make log dir
  mkdir -p "${LOG_DIR}"

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
  main
fi
