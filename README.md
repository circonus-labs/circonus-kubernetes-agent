# Circonus Kubernetes Agent

An agent designed to retrieve metrics from a Kubernetes cluster. Runs as a deployment, forwards Kubernetes provided metrics for cluster, nodes, pods, and containers to Circonus.

>NOTE: in active development, not all features are guaranteed to be complete.

## Installation

## Prerequisites

### `kubectl` (default)

1. For full functionality [kube-state-metrics](https://github.com/kubernetes/kube-state-metrics) should be installed in the cluster

#### Default (simple)

1. Clone repo
1. In `deploy/default/configuration.yaml` set the following required attributes:
   * Set Circonus API Token - `circonus-api-key`
   * Kubernetes Cluster Name - `kubernetes-name`
   * Circonus Alert Email - `default-alerts.json`->`contact.email` - email address for default alerts
1. Apply `kubectl apply -f deploy/default/`

#### Custom

1. Clone repo
1. Verify `deploy/custom/authrbac.yaml`, alter any applicable settings for cluster security
1. Change any applicable settings in `deploy/custom/configuration.yaml`, minimum required:
   * Circonus API Token
   * Check target - so agent can find check on restart (short, unique string w/o spaces - normally this is an FQDN)
   * Kubernetes name - used for check title when creating a check
   * Circonus Alert Email - email address for default alerts
   * For full functionality [kube-state-metrics](https://github.com/kubernetes/kube-state-metrics) should be installed in the cluster
1. Change any applicable settings in `deploy/custom/deployment.yaml`
1. Apply `kubectl apply -f deploy/custom/`

### `helm` (contrib)

1. Clone repo
1. Make updates to files in `contrib/helm` to customize settings
1. Install

```sh
helm install contrib/helm \
  --name=<Helm release name> \
  --set=circonus_api_key=<some valid Circonus API key> \
  --set=kubernetes_name=<kubernetes-name> \
  --set=circonus_check_target=<circonus-check-target> \
  --wait
```

## Versions

Developed against and tested with...

* kubernetes v1.17.0
* etcd v3.4.3
* calico v3.10
* kube-state-metrics v1.7.2 (arm) and v1.8.0 (amd)
