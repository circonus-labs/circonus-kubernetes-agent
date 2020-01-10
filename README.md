# Circonus Kubernetes Agent

An agent designed to retrieve metrics from a Kubernetes cluster. Runs as a deployment, forwards Kubernetes provided metrics for cluster, nodes, pods, and containers to Circonus.

>NOTE: in active development, not all features are guaranteed to be complete.

## Installation

1. Clone repo
1. Verify `deploy/authrbac.yaml`, alter any applicable settings for cluster security
1. Change any applicable settings in `deploy/configuration.yaml`, minimum required:
   * Circonus API Token
   * check target - so the agent can find the check on restart (short, unique string w/o spaces - normally this is an FQDN)
   * Kubernetes name - used for check title when creating a check
1. Change any applicable settings in `deploy/deployment.yaml`
1. Apply `kubectl apply -f deploy/`

## Versions

Developed against and tested with...

* kubernetes v1.17.0
* etcd v3.4.3
* calico v3.10
* metrics-server v0.3.6
* kube-state-metrics v1.7.2 (arm) and v1.8.0 (amd)
