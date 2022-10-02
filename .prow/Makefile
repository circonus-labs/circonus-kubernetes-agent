MULTIFORM=./tf/gcp/multiform.sh
KUBECONFIG=$(HOME)/.kube/config

SHELL=/usr/bin/env bash

.PHONY: all

all: terraform helm

terraform: gcp

gcp:
	$(MULTIFORM) apply
	$(MULTIFORM) watch_apply
	if [ -f $(KUBECONFIG) ]; then mv $(KUBECONFIG) $(KUBECONFIG).bak; fi
	$(MULTIFORM) kubeconfig
	$(MULTIFORM) proxy

helm:
	./helm/admiral.sh

clean:
	if [ -f $(KUBECONFIG).bak ]; then mv $(KUBECONFIG).bak $(KUBECONFIG); fi
	$(MULTIFORM) teardown_proxy
	$(MULTIFORM) destroy
	$(MULTIFORM) watch_destroy

