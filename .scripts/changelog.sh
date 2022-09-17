#!/usr/bin/env bash

cd "$(dirname """$0""")"/../ || exit

# usage: changelog.sh [ -c | --changelog-data CHANGELOG_DATA_FILE ] [ -d | --debug ] [ -f | --from-version FROM_VERSION ] [ -o | --output OUTPUT_FILE ] [ -t | --to-version TO_VERSION ]

# Prior to generating a release
# use TO_VERSION/-t/--to-version, and the util

# requirements:
# - chglog (https://github.com/goreleaser/chglog)
# - yq (https://github.com/mikefarah/yq)

# @default: "CHANGELOG.md"
# @type: string
OUTPUT_FILE="${OUTPUT_FILE:-CHANGELOG.md}"

# @default: "CHANGELOG.md.tmpl"
# @type: string
OUTPUT_TEMPLATE_FILE="${OUTPUT_TEMPLATE_FILE:-./templates/files/${OUTPUT_FILE}.tmpl}"

# @default: @generated
# @type: string
# @note: defaults to the last version in the OUTPUT_FILE (all if empty)
# @note: This is the older version
# @note: FROM_VERSION is the previous version in the changelog and can be automatically detected from the existing OUTPUT_FILE
FROM_VERSION="${FROM_VERSION:-}"

# @default: @generated
# @type: string
# @note: defaults to the latest tagged version
# @note: This is the newer version
# @note: TO_VERSION is the new version to be added to the changelog and can be automatically detected from the existing CHANGELOG_DATA_FILE
TO_VERSION="${TO_VERSION:-}"

# @default: @generated
# @type: string
# @note: defaults to the latest tagged version + 1 minor version
SUGGESTED_VERSION="$(grep v CHANGELOG.md | head -1 | cut -f2 -d' ' | awk -F. -v OFS=. '{$NF += 1 ; print}')"

# @default: false
# @type: boolean
DEBUG="${DEBUG:-false}"

# @default: changelog.yml
# @type: string
CHANGELOG_DATA_FILE="${CHANGELOG_DATA_FILE:-changelog.yml}"


#
# parse arguments
#
POSITIONALS=""
while (( "${#}" )); do
  case "${1}" in
    -d|--debug)
      DEBUG=true
      shift
      ;;
    -c|--changelog-data)
      if [ -n "${2}" ] && [ "${2:0:1}" != "-" ]; then
        CHANGELOG_DATA_FILE=${2}
        shift 2
      else
        echo "ERROR: Argument for ${1} is missing" >&2
        echo "Exiting..."
        exit 1
      fi
      ;;
    -f|--from-version)
      if [ -n "${2}" ] && [ "${2:0:1}" != "-" ]; then
        FROM_VERSION=${2}
        shift 2
      else
        echo "ERROR: Argument for ${1} is missing" >&2
        echo "Exiting..."
        exit 1
      fi
      ;;
    -o|--output)
      if [ -n "${2}" ] && [ "${2:0:1}" != "-" ]; then
        OUTPUT_FILE=${2}
        shift 2
      else
        echo "ERROR: Argument for ${1} is missing" >&2
        echo "Exiting..."
        exit 1
      fi
      ;;
    -t|--to-version)
      if [ -n "${2}" ] && [ "${2:0:1}" != "-" ]; then
        TO_VERSION=${2}
        shift 2
      else
        echo "ERROR: Argument for ${1} is missing" >&2
        echo "Exiting..."
        exit 1
      fi
      ;;
    -*) # unsupported flags
      echo "ERROR: Unsupported flag ${1}" >&2
      echo "Exiting..."
      exit 1
      ;;
    *) # preserve positional arguments
      POSITIONALS="${POSITIONALS} ${1}"
      shift
      ;;
  esac
done
# set positionals back in their place
eval set -- "${POSITIONALS}"
#
# parse arguments
#


# if CHANGELOG_DATA_FILE doesn't exist, create it
if [ ! -f "${CHANGELOG_DATA_FILE}" ]; then
  if [ -n "${OUTPUT_FILE}" ]; then
    chglog init \
      --output "${CHANGELOG_DATA_FILE}"
  # if CHANGELOG_DATA_FILE is unset, create the default file
  else
    chglog init
  fi
fi

# if OUTPUT_FILE does not exist and OUTPUT_FILE is not "" or unset, create it.
if [ ! -f "${OUTPUT_FILE}" ] && [ -n "${OUTPUT_FILE}" ] && [ "${DEBUG}" = "false" ]; then
  touch "${OUTPUT_FILE}"
fi

# if FROM_VERSION is unset, obtain it from OUTPUT_FILE (if possible)
if [ -z "${FROM_VERSION}" ]; then
  # if OUTPUT_FILE exists, try to get FROM_VERSION from it
  if [ -f "${OUTPUT_FILE}" ]; then
    FROM_VERSION=$(grep "# v" "${OUTPUT_FILE}" | head -n1 | sed -e "s/# //g")
  else
    echo "WARN: output file does not exist and FROM_VERSION is not set."
    echo "INFO: using v0.0.0 as FROM_VERSION"
    FROM_VERSION="v0.0.0"
  fi
fi

# if TO_VERSION is unset, prompt the user
if [ -z "${TO_VERSION}" ]; then
  echo "From version is: ${FROM_VERSION}"
  echo "Suggested version is: ${SUGGESTED_VERSION}"
  while true; do
    read -r -p "Press [Y/y] to accept suggesed version or input a semantic version: " to_version
    case $to_version in
      [Yy]* ) TO_VERSION="${SUGGESTED_VERSION}"; break;;
      v* ) TO_VERSION="${to_version}"; break;;
    esac
  done
fi

# if TO_VERSION doesn't exist in the CHANGELOG_DATA_FILE, add it
CHANGELOG_DATA_FILE_LATEST_TAG=$(yq '.[0].semver' "${CHANGELOG_DATA_FILE}")
CHANGELOG_DATA_FILE_LATEST_TAG="v${CHANGELOG_DATA_FILE_LATEST_TAG}"
if [ "${CHANGELOG_DATA_FILE_LATEST_TAG}" != "${TO_VERSION}" ] ; then
  if [ "${DEBUG}" = "false" ]; then
    if [ -f "${CHANGELOG_DATA_FILE}" ]; then
      chglog add \
        --version "${TO_VERSION}" \
        --input "${CHANGELOG_DATA_FILE}" \
        --output "${CHANGELOG_DATA_FILE}"
    else
      chglog add \
        --version "${TO_VERSION}"
    fi
  fi
fi

# if the user misconfigured the script, exit
if [ "${FROM_VERSION}" = "${TO_VERSION}" ]; then
  echo "ERROR: From version: ${FROM_VERSION} is the same as to version: ${TO_VERSION}."
  echo "Exiting..."
  exit 1
fi

# if not debug, check for and remove old entries in the changelog between TO_VERSION and FROM_VERSION.
if [ "${DEBUG}" = "false" ]; then
  sed -n -i '' -e '/'"${TO_VERSION}"'/{' -e ':a' -e 'N' -e '/'"${FROM_VERSION}"'/!ba' -e 's/.*\n//' -e '}' -e 'p' "${OUTPUT_FILE}"
fi

# create copy of old changelog for writing
if [ "${DEBUG}" = "false" ]; then
  cp "${OUTPUT_FILE}" ".${OUTPUT_FILE}"
fi

# output formatted changelog
if [ "${DEBUG}" = "false" ]; then
  chglog format \
    --input "${CHANGELOG_DATA_FILE}" \
    --template-file "${OUTPUT_TEMPLATE_FILE}" \
    --output ".${OUTPUT_FILE}-chglog"
else
  chglog format \
    --input "${CHANGELOG_DATA_FILE}" \
    --template-file "${OUTPUT_TEMPLATE_FILE}" | \
    tee ".${OUTPUT_FILE}-chglog"
fi

# grab new changes from formatted changelog, add to the top of copy of old changelog
CHANGES=$(sed -n '/^# '"${TO_VERSION}"'$/,/^# '"${FROM_VERSION}"'$/p' ".${OUTPUT_FILE}-chglog" | sed '$d')

# grab CHANGES and add to OUTPUT_FILE, then add old OUTPUT_FILE 
if [ "${DEBUG}" = "false" ]; then
  echo "${CHANGES}" > "${OUTPUT_FILE}"
  echo "" >> "${OUTPUT_FILE}"
  cat ".${OUTPUT_FILE}" >> "${OUTPUT_FILE}"
else
  echo "### CHANGES ###"
  echo "${CHANGES}"
  echo "### END CHANGES ###"
fi

# remove temporary files
if [ "${DEBUG}" = "false" ]; then
  rm ".${OUTPUT_FILE}" 
fi
rm ".${OUTPUT_FILE}-chglog"

# output variables for debug
if [ "${DEBUG}" = "true" ]; then
  echo "OUTPUT_FILE=${OUTPUT_FILE}"
  echo "FROM_VERSION=${FROM_VERSION}"
  echo "TO_VERSION=${TO_VERSION}"
  echo "POSITIONALS=${POSITIONALS}"
  echo "CHANGELOG_DATA_FILE=${CHANGELOG_DATA_FILE}"
  echo "OUTPUT_TEMPLATE_FILE=${OUTPUT_TEMPLATE_FILE}"
fi

