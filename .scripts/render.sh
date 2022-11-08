#!/usr/bin/env bash

# usage: render.sh [ all | code | manifests | ... ] ...
# command line arguments reference functions defined in this file.

# requirements:
# - render (https://github.com/VirtusLab/render)
# - yq ()

# cd to repo's top-level dir
cd "$(dirname """$0""")/../" || exit


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
TEMPLATE_DATA_FILE="${TEMPLATE_DATA_FILE:-$TEMPLATE_DIR/data/data.yaml}"

# @default: CHANGELOG.md
# @type: string
CHANGELOG_FILE="${CHANGELOG_FILE:-CHANGELOG.md}"

render_file() {

  FILE_NAME="${1}"
  if [ "${FILE_NAME: -5}" != ".tmpl" ]; then
    echo "FILE: ${FILE_NAME} does not end in .tmpl"
    return 1
  fi

  if [ "${FILE_NAME}" = "./templates/files/CHANGELOG.md.tmpl" ]; then
    return 0
  fi

  FILE_NAME="${FILE_NAME#"${TEMPLATE_FILES_DIR}"/}"
  FILE_NAME="${FILE_NAME%.tmpl}"

  if [ -z "${FILE_NAME}" ]; then
    echo "${0} called with no file name specified"
  fi
  render --in "${TEMPLATE_FILES_DIR}/${FILE_NAME}.tmpl" --out "${FILE_NAME}" --config "${TEMPLATE_DATA_FILE}" 1>/dev/null
}

all() {

  # set changelog version from CHANGELOG file
  CHANGELOG_VERSION="$(grep v CHANGELOG.md | head -1 | cut -f2 -d' ')"

  # set agent version in yaml from changelog version
  yq -i '.data.agent_version = "'"${CHANGELOG_VERSION}"'"' "${TEMPLATE_DATA_FILE}"

  # hacky, shellcheck made me do
  export -f render_file
  export TEMPLATE_FILES_DIR
  export TEMPLATE_DATA_FILE
  find "${TEMPLATE_FILES_DIR}" -type f -exec sh -c 'render_file "$1"' shell {} \;
}

# Parse command line arguments
if [ "$#" -eq 0 ]; then
  ${DEFAULT_COMMAND}
else
  for cmd in "$@"; do
    $cmd
  done
fi
