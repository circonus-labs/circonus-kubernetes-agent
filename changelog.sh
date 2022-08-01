#!/usr/bin/env bash

# usage: changelog.sh [ -c | --changelog-data CHANGELOG_DATA_FILE ] [ -d | --debug ] [ -f | --from-version FROM_VERSION ] [ -o | --output OUTPUT_FILE ] [ -t | --to-version TO_VERSION ]

# FROM_VERSION is the previous version in the changelog and can be automatically detected from the existing OUTPUT_FILE
# TO_VERSION is the new version to be added to the changelog and can be automatically detected from the existing CHANGELOG_DATA_FILE
# Optionally, it can be specified and the tool will automatically add it to the CHANGELOG_DATA_FILE and OUTPUT_FILE
# requirements:
# - chglog (https://github.com/goreleaser/chglog)
# - yq (https://github.com/mikefarah/yq)

# SOP:
# Once a PR is ready to be merged, either: 
# run `chglog add --version vX.Y.Z` and DO NOT SPECIFY TO_VERSION/-t/--to-version
# or 
# use TO_VERSION/-t/--to-version, and the utility will do it for you

# @default: "CHANGELOG.md"
# @type: string
OUTPUT_FILE="${OUTPUT_FILE:-CHANGELOG.md}"

# @default: "CHANGELOG.md.tmpl"
# @type: string
OUTPUT_TEMPLATE_FILE="${OUTPUT_TEMPLATE_FILE:-${OUTPUT_FILE}.tmpl}"

# @default: @generated
# @type: string
# @note: defaults to the last version in the OUTPUT_FILE (all if empty)
# @note: This is the older version
FROM_VERSION="${FROM_VERSION:-}"

# @default: @generated
# @type: string
# @note: defaults to the latest tagged version
# @note: This is the newer version
TO_VERSION="${TO_VERSION:-}"

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

# if TO_VERSION is unset, exit
if [ -z "${TO_VERSION}" ]; then
  if [ -f "${CHANGELOG_DATA_FILE}" ]; then
    TO_VERSION=$(yq '.[0].semver' "${CHANGELOG_DATA_FILE}")
  else
    echo "ERROR: to version unset and no CHANGELOG_DATA_FILE found."
    echo "Exiting..."
    exit 1
  fi
fi

# if TO_VERSION doesn't exist in the CHANGELOG_DATA_FILE, add it
CHANGELOG_DATA_FILE_LATEST_TAG=$(yq '.[0].semver' "${CHANGELOG_DATA_FILE}")
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
    --template-file "${OUTPUT_TEMPLATE_FILE}"
fi

# grab new changes from formatted changelog, add to the top of copy of old changelog
# shellcheck disable=2016 # code is irrelevant because ${p is a sed command and shouldn't be expanded
CHANGES=$(sed -i'' -n '/^'"${TO_VERSION}"'/,${p;/^'"${FROM_VERSION}"'/q}' "${OUTPUT_FILE}-chglog")

# grab CHANGES and add to OUTPUT_FILE, then add old OUTPUT_FILE 
if [ "${DEBUG}" = "false" ]; then
  echo "${CHANGES}" > "${OUTPUT_FILE}"
  cat ".${OUTPUT_FILE}" >> "${OUTPUT_FILE}"
else
  echo "${CHANGES}"
fi

# remove temporary files
if [ "${DEBUG}" = "false" ]; then
  rm ".${OUTPUT_FILE}" "${OUTPUT_FILE}-chglog"
fi

# output variables for debug
if [ "${DEBUG}" = "true" ]; then
  echo "OUTPUT_FILE=${OUTPUT_FILE}"
  echo "FROM_VERSION=${FROM_VERSION}"
  echo "TO_VERSION=${TO_VERSION}"
  echo "POSITIONALS=${POSITIONALS}"
  echo "CHANGELOG_DATA_FILE=${CHANGELOG_DATA_FILE}"
  echo "OUTPUT_TEMPLATE_FILE=${OUTPUT_TEMPLATE_FILE}"
fi

