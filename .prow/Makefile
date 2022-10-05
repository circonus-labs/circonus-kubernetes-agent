ADMIRAL=./helm/admiral.sh
DATA_FILE=./supported-k8s-versions.yaml
KUBECONFIG=$(HOME)/.kube/config
MULTIFORM=./tf/gcp/multiform.sh

SHELL=/usr/bin/env bash

# CI systems should use `IS_CI=true make ...`
ifdef IS_CI
# Defaults NAME_PREFIX to the name of the current branch if IS_CI is true
NAME_PREFIX=$(shell git symbolic-ref --short HEAD)
endif

ifeq ($(NAME_PREFIX),)
# Defaults NAME_PREFIX to "manual" if IS_CI is unset or set to ""
NAME_PREFIX=manual
endif

RUNTIME_DATA_FILE=./.runtime-$(NAME_PREFIX).yaml

.PHONY: all terraform gcp runtime_data helm clean

all: terraform helm

terraform: gcp

gcp: runtime_data
	NAME_PREFIX=$(NAME_PREFIX) $(MULTIFORM) apply
# ifeq ($(NAME_PREFIX),manual)
# 	NAME_PREFIX=$(NAME_PREFIX) $(MULTIFORM) watch_apply
# else
	NAME_PREFIX=$(NAME_PREFIX) $(MULTIFORM) wait_apply
# endif
	if [ -f "$(KUBECONFIG)" ]; then mv $(KUBECONFIG) $(KUBECONFIG).bak; fi
	NAME_PREFIX=$(NAME_PREFIX) $(MULTIFORM) kubeconfig
	NAME_PREFIX=$(NAME_PREFIX) $(MULTIFORM) proxy

runtime_data:
ifeq ($(NAME_PREFIX),manual)
	if [ ! -f "$(RUNTIME_DATA_FILE)" ]; then cp "$(DATA_FILE)" "$(RUNTIME_DATA_FILE)"; fi
else
	cp "$(DATA_FILE)" "$(RUNTIME_DATA_FILE)"
endif

helm:
	NAME_PREFIX=$(NAME_PREFIX) $(ADMIRAL) sync
# ifeq ($(NAME_PREFIX),manual)
#   NAME_PREFIX=$(NAME_PREFIX) $(ADMIRAL) watch_sync
# else
	NAME_PREFIX=$(NAME_PREFIX) $(ADMIRAL) wait_sync
# endif

clean:
	if [ -f "$(KUBECONFIG).bak" ]; then mv "$(KUBECONFIG).bak" "$(KUBECONFIG)"; fi
	$(MULTIFORM) teardown_proxy
	$(MULTIFORM) destroy
ifeq ($(NAME_PREFIX),manual)
	$(MULTIFORM) wait_destroy
#	$(MULTIFORM) watch_destroy
else
	$(MULTIFORM) wait_destroy
	rm -f "$(RUNTIME_DATA_FILE)"
endif
