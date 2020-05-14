# Circonus Kubernetes Agent

An agent designed to retrieve metrics from a Kubernetes cluster. Runs as a deployment, forwards Kubernetes provided metrics for cluster, nodes, pods, and containers to Circonus.

>NOTE: in active development, not all features are guaranteed to be complete.

## Installation

### `kubectl`
1. Clone repo
1. Verify `deploy/authrbac.yaml`, alter any applicable settings for cluster security
1. Change any applicable settings in `deploy/configuration.yaml`, minimum required:
   * Circonus API Token
   * check target - so the agent can find the check on restart (short, unique string w/o spaces - normally this is an FQDN)
   * Kubernetes name - used for check title when creating a check
   * It is recommended that [kube-state-metrics](https://github.com/kubernetes/kube-state-metrics) be installed in the cluster and collection enabled in the configuration for all dashboard tabs to function
1. Change any applicable settings in `deploy/deployment.yaml`
1. Apply `kubectl apply -f deploy/*.yaml`

### `helm`

```
helm install deploy/helm \
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
