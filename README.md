# Circonus Kubernetes Agent

An agent designed to retrieve metrics from a Kubernetes cluster. Runs as a deployment, forwards Kubernetes provided metrics for cluster, nodes, pods, and containers to Circonus.

## Prerequisites

### kube-state-metrics

For full functionality [kube-state-metrics](https://github.com/kubernetes/kube-state-metrics) should be installed in the cluster

### DNS

For DNS metrics the agent will look for a service named `kube-dns` in the `kube-system` namespace. It will check for two annotations `prometheus.io/scrape` and `prometheus.io/port`. If `scrape` is `true`, the agent will collect metrics from the endpoints listed for the target port. For example:

```yaml
apiVersion: v1
kind: Service
metadata:
  annotations:
    prometheus.io/port: "9153"
    prometheus.io/scrape: "true"
  labels:
    k8s-app: kube-dns
    kubernetes.io/cluster-service: "true"
    kubernetes.io/name: KubeDNS
  name: kube-dns
  namespace: kube-system
spec:
  selector:
    k8s-app: kube-dns
  type: ClusterIP
  ports:
  - name: dns
    port: 53
    protocol: UDP
    targetPort: 53
  - name: dns-tcp
    port: 53
    protocol: TCP
    targetPort: 53
  - name: metrics
    port: 9153
    protocol: TCP
    targetPort: 9153
```

> NOTE: if the annotations are _not_ defined on the service, and the service cannot be modfied for any reason. If the pods backing the kube-dns service _do_ expose metrics use `kube-dns-metrics-port` to define the port from which to request `/metrics`.

## Installation

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

#### Observation

> NOTE: only use this deployment as requested by Circonus

1. Clone repo
1. In `deploy/observation/configuration.yaml` set the following required attributes:
   * Circonus API Token - `circonus-api-key`
   * Kubernetes Cluster Name - `kubernetes-name` - short, unique string w/o spaces
1. Apply `kubectl apply -f deploy/observation/`

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
