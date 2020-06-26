# Circonus Kubernetes Agent

An agent designed to retrieve metrics from a Kubernetes cluster. Runs as a deployment, forwards Kubernetes provided metrics for cluster, nodes, pods, and containers to Circonus.

## Installation

### Prerequisites

1. For full functionality [kube-state-metrics](https://github.com/kubernetes/kube-state-metrics) should be installed in the cluster

### `kubectl`

#### Default (simple)

1. Clone repo
1. In `deploy/default/configuration.yaml` set the following required attributes:
   * Circonus API Token - `circonus-api-key`
   * Kubernetes Cluster Name - `kubernetes-name` - short, unique string w/o spaces
   * Circonus Alert Email - `default-alerts.json`->`contact.email` - email address for default alerts
1. Apply `kubectl apply -f deploy/default/`

#### Custom

1. Clone repo
1. Verify `deploy/custom/authrbac.yaml`, alter any applicable settings for cluster security
1. Change any applicable settings in `deploy/custom/configuration.yaml`, minimum required:
   * Circonus API Token
   * Check Target (optional, Kubernetes cluster name will be used if not supplied) - so agent can find check on restart (short, unique string w/o spaces - normally this is a FQDN)
   * Kubernetes Cluster Name - used for check title when creating a check
   * Circonus Alert Email - email address for default alerts
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
