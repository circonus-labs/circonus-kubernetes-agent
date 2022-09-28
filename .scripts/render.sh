#!/usr/bin/env bash

# usage: render.sh [ all | code | manifests | ... ] ...
# command line arguments reference functions defined in this file.

# requirements:
# - render (https://github.com/VirtusLab/render)

# @default: all
# @type: string
DEFAULT_COMMAND="${DEFAULT_COMMAND:-all}"

# @default: ./templates
# @type: string
TEMPLATE_DIR="${TEMPLATE_DIR:-./templates}"

# @default: $TEMPLATE_DIR/files
# @type: string
TEMPLATE_FILES_DIR="${TEMPLATE_FILES_DIR:-$TEMPLATE_DIR/files}"

# @default: $TEMPLATE_DIR/data
# @type: string
TEMPLATE_DATA_DIR="${TEMPLATE_DATA_DIR:-$TEMPLATE_DIR/data}"

code() {
  FILE_NAME="internal/circonus/metric_filters.go"
  render --in "${TEMPLATE_FILES_DIR}/${FILE_NAME}.tmpl" --out "./${FILE_NAME}" --config "${TEMPLATE_DATA_DIR}/metric_filters.yaml"
}

manifests() {
  FILE_NAME="deploy/custom/configuration.yaml"
  render --in "${TEMPLATE_FILES_DIR}/${FILE_NAME}.tmpl" --out "./${FILE_NAME}" --config "${TEMPLATE_DATA_DIR}/metric_filters.yaml"

  FILE_NAME="deploy/custom/deployment.yaml"
  render --in "${TEMPLATE_FILES_DIR}/${FILE_NAME}.tmpl" --out "./${FILE_NAME}" --config <( echo "version: $(grep v CHANGELOG.md | head -1 | cut -f2 -d' ')" )
}

all() {
  code
  manifests
}

# cd to repo's top-level dir
cd "$(dirname """$0""")/../" || exit

# Parse command line arguments
if [ "$#" -eq 0 ]; then
  ${DEFAULT_COMMAND}
else
  for cmd in "$@"; do
    $cmd
  done
fi
