#!/usr/bin/env bash

echo
echo "Running GO Pull Request Lint (Linux specific)"
GOOS=linux golangci-lint run || exit 1

