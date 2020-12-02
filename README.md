# Circonus Kubernetes Agent

An agent designed to retrieve metrics from a Kubernetes cluster. Runs as a deployment, forwards Kubernetes provided metrics for cluster, nodes, pods, and containers to Circonus.

---

## Table of contents

* [Prerequisites](#prerequisites)
  * [kube-state-metrics](#kube-state-metrics)
  * [DNS](#dns)
* [Installation](#installation)
  * [kubectl](#kubectl)
    * [Default](#default)
    * [Custom](#custom)
    * [Observation](#observation)
  * [helm](#helm) - contributed example helm chart **will require modfiication**
* [Options](#options) - command line parameters
* [Dynamic collection](#dynamic-collection) - endpoints, services, pods, and nodes
  * [Configuration options](#configuration-options)
  * [Examples](#examples)

---

## Prerequisites

### kube-state-metrics

For full functionality a recent version of [kube-state-metrics](https://github.com/kubernetes/kube-state-metrics) should be installed in the cluster - see [kube-state-metrics deployment instructions](https://github.com/kubernetes/kube-state-metrics#kubernetes-deployment) for more information.

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

## Options

```text
Usage:
  circonus-kubernetes-agent [flags]

Flags:
      --api-app string                        [ENV: CKA_CIRCONUS_API_APP] Circonus API Token App name (default "circonus-kubernetes-agent")
      --api-cafile string                     [ENV: CKA_CIRCONUS_API_CAFILE] Circonus API CA file
      --api-debug                             [ENV: CKA_CIRCONUS_API_DEBUG] Debug Circonus API calls
      --api-key string                        [ENV: CKA_CIRCONUS_API_KEY] Circonus API Token Key
      --api-key-file string                   [ENV: CKA_CIRCONUS_API_KEY_FILE] Circonus API Token Key file
      --api-url string                        [ENV: CKA_CIRCONUS_API_URL] Circonus API URL (default "https://api.circonus.com/v2/")
      --check-broker-ca-file string           [ENV: CKA_CIRCONUS_CHECK_BROKER_CAFILE] Circonus Check Broker CA file
      --check-broker-cid string               [ENV: CKA_CIRCONUS_CHECK_BROKER_CID] Circonus Check Broker CID to use when creating a check (default "/broker/35")
      --check-bundle-cid string               [ENV: CKA_CIRCONUS_CHECK_BUNDLE_CID] Circonus Check Bundle CID
      --check-create                          [ENV: CKA_CIRCONUS_CHECK_CREATE] Circonus Create Check if needed (default true)
      --check-tags string                     [ENV: CKA_CIRCONUS_CHECK_TAGS] Circonus Check Tags
      --check-target string                   [ENV: CKA_CIRCONUS_CHECK_TARGET] Circonus Check Target host
      --check-title string                    [ENV: CKA_CIRCONUS_CHECK_TITLE] Circonus check display name (default: <--k8s-name> /circonus-kubernetes-agent)
  -c, --config string                         config file (default: /etc/circonus-kubernetes-agent.yaml|.json|.toml)
      --custom-rules-file string              [ENV: CKA_CIRCONUS_CUSTOM_RULES_FILE] Circonus custom rules configuration file (default "/ck8sa/custom-rules.json")
  -d, --debug                                 [ENV: CKA_DEBUG] Enable debug messages
      --default-alerts-file string            [ENV: CKA_CIRCONUS_DEFAULT_ALERTS_FILE] Circonus default alerts configuration file (default "/ck8sa/default-alerts.json")
      --default-streamtags string             [ENV: CKA_CIRCONUS_DEFAULT_STREAMTAGS] Circonus default streamtags for all metrics
  -h, --help                                  help for circonus-kubernetes-agent
      --k8s-api-cafile string                 [ENV: CKA_K8S_API_CAFILE] Kubernetes API CA File (default "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt")
      --k8s-api-timelimit string              [ENV: CKA_K8S_API_TIMELIMIT] Kubernetes API request timelimit (default "10s")
      --k8s-api-url string                    [ENV: CKA_K8S_API_URL] Kubernetes API URL (default "https://kubernetes.default.svc")
      --k8s-bearer-token string               [ENV: CKA_K8S_BEARER_TOKEN] Kubernetes Bearer Token
      --k8s-bearer-token-file string          [ENV: CKA_K8S_BEARER_TOKEN_FILE] Kubernetes Bearer Token File (default "/var/run/secrets/kubernetes.io/serviceaccount/token")
       --k8s-dynamic-collector-file string     [ENV: CKA_K8S_DYNAMIC_COLLECTOR_FILE] Kubernetes dynamic collectors configuration file (default "/ck8sa/dynamic-collectors.json")
      --k8s-enable-api-server                 [ENV: CKA_K8S_ENABLE_API_SERVER] Kubernetes enable collection from api-server (default true)
      --k8s-enable-cadvisor-metrics           [ENV: CKA_K8S_ENABLE_CADVISOR_METRICS] Kubernetes enable collection of kubelet cadvisor metrics
      --k8s-enable-events                     [ENV: CKA_K8S_ENABLE_EVENTS] Kubernetes enable collection of events (default true)
      --k8s-enable-kube-dns-metrics           [ENV: CKA_K8S_ENABLE_KUBE_DNS_METRICS] Kubernetes enable collection of kube dns metrics (default true)
      --k8s-enable-kube-state-metrics         [ENV: CKA_K8S_ENABLE_KUBE_STATE_METRICS] Kubernetes enable collection from kube-state-metrics (default true)
      --k8s-enable-metrics-server             [ENV: CKA_K8S_ENABLE_METRICS_SERVER] Kubernetes enable collection from metrics-server
      --k8s-enable-node-metrics               [ENV: CKA_K8S_ENABLE_NODE_METRICS] Kubernetes include metrics for individual nodes (default true)
      --k8s-enable-node-stats                 [ENV: CKA_K8S_ENABLE_NODE_STATS] Kubernetes include summary stats for individual nodes (and pods) (default true)
      --k8s-enable-nodes                      [ENV: CKA_K8S_ENABLE_NODES] Kubernetes include metrics for individual nodes (default true)
      --k8s-include-containers                [ENV: CKA_K8S_INCLUDE_CONTAINERS] Kubernetes include metrics for individual containers
      --k8s-include-pods                      [ENV: CKA_K8S_INCLUDE_PODS] Kubernetes include metrics for individual pods (default true)
      --k8s-interval string                   [ENV: CKA_K8S_INTERVAL] Kubernetes Cluster collection interval (default "1m")
      --k8s-ksm-field-selector-query string   [ENV: CKA_K8S_KSM_FIELD_SELECTOR_QUERY] Kube-state-metrics fieldSelector query for finding the correct KSM installation (default "metadata.name=kube-state-metrics")
      --k8s-ksm-metrics-port-name string      [ENV: CKA_K8S_KSM_METRICS_PORT_NAME] Kube-state-metrics metrics port name (default "http-metrics")
      --k8s-ksm-request-mode string           [ENV: CKA_K8S_KSM_REQUEST_MODE] Kube-state-metrics request mode, proxy or direct (default "direct")
      --k8s-ksm-telemetry-port-name string    [ENV: CKA_K8S_KSM_TELEMETRY_PORT_NAME] Kube-state-metrics telemetry port name (default "telemetry")
      --k8s-kube-dns-metrics-port string      [ENV: CKA_K8S_KUBE_DNS_METRICS_PORT] Kube dns metrics port if annotations not on service definition (default "10054")
      --k8s-name string                       [ENV: CKA_K8S_NAME] Kubernetes Cluster Name (used in check title)
      --k8s-node-selector string              [ENV: CKA_K8S_NODE_SELECTOR] Kubernetes key:value node label selector expression
      --k8s-pod-label-key string              [ENV: CKA_K8S_POD_LABEL_KEY] Include pods with label
      --k8s-pod-label-val string              [ENV: CKA_K8S_POD_LABEL_VAL] Include pods with pod label and matching value
      --k8s-pool-size uint                    [ENV: CKA_K8S_POOL_SIZE] Kubernetes node collector pool size (default 4)
      --log-level string                      [ENV: CKA_LOG_LEVEL] Log level [(panic|fatal|error|warn|info|debug|disabled)] (default "info")
      --log-pretty                            Output formatted/colored log lines [ignored on windows]
      --metric-filters-file string            [ENV: CKA_CIRCONUS_METRIC_FILTERS_FILE] Circonus Check metric filters configuration file (default "/ck8sa/metric-filters.json")
      --show-config string                    Show config (json|toml|yaml) and exit
      --trace-submits string                  Trace metrics submitted to Circonus to passed directory (one file per submission)
  -V, --version                               Show version and exit
```

## Dynamic collection

Dynamic collection simplifies gathering desirable metrics exposed in Prometheus format from endpoints, services, pods, and nodes.

1. Use [Custom](#custom) deployment in order to configure dynamic collectors
1. Add metric filters to the configuration for the desired metrics from dynamic collectors.

### Configuraiton options

The configuraiton is defined in the [custom](deploy/custom/configuration.yaml) deployment.

```yaml
collectors:
  - name: ""           # required
    disable: false     # disable this collector
    type: ""           # required - endpoints, nodes, pods, services
    schema:            # http or https
      annotation: ""   # use the value of an annotation
      label: ""        # use the value of a label
      value: ""        # use a static value
    selectors:         # defaults to all of the type
      label: ""        # labelSelector expression
      field: ""        # fieldSelector expression
    control:           # control whether to collect metrics from specific item
      annotation: ""   # compare annotation setting against value
      label: ""        # compare label setting against value
      value: ""        # value to use in comparisson to annotation or label
    metric_port:       # define port to use in metric request
      annotation: ""   # use the value of an annotation
      label: ""        # use the value of a label
      value: ""        # use a static value
    metric_path:       # define path to use in metric request, default `/metrics`
      annotation: ""   # use the value of an annotation
      label: ""        # use the value of a label
      value: ""        # use a static value
    tags: ""           # comma separated list of static tags to add
    label_tags: ""     # comma separated list of labels on the item to add as tags
    rollup: false      # rollup metrics
```

| option | required | description | default |
| ------- | ---------| ----------- | ------- |
| name | yes | name of this collector | n/a |
| disable | no | disable a collector, but keep the configuration | false |
| type | yes | type of the collector (`endpoints`, `nodes`, `pods`, `services`) | n/a |
| schema | no | HTTP request schema `http` or `https` ||
| schema.annotation | no | annotation to use ||
| schema.label | no | label to use ||
| schema.value | no | static schema for all instances | `http` |
| selectors || define what items of the type to collect ||
| selectors.label | no | a [labelSelector](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#label-selectors) for the type | all for type |
| selectors.field | no | a [fieldSelector](https://kubernetes.io/docs/concepts/overview/working-with-objects/field-selectors/) for the type | all for type |
| control || controls collection of metrics at instance level ||
| control.annotation | no | annotation to use e.g. `"monitor"` ||
| control.label | no | label to use  e.g. `"metricsCollect"` ||
| control.value | no | value the annotation or label should be for collection e.g. `"true"` ||
| metric_port || defines the port to use for the metric request ||
| metric_port.annotation | no | annotation to use ||
| metric_port.label | no | label to use ||
| metric_port.value | no | static port for all instances ||
| metric_path || defines the path to use for the metric request ||
| metric_path.annotation | no | annotation to use ||
| metric_path.label | no | label to use ||
| metric_path.value | no | static path to use for all instances | `/metrics`|
| tags | no | comma separated list of static tags to add e.g. `"app:myapp,foo:bar"` ||
| label_tags | no | comma separated list of labels to use as tags e.g. `"environment,location"` ||
| rollup | no | true or false, control rolling up metrics | false |

### Examples

From `endpoints` with the label `kubernetes.io/name=KubeDNS` collect metrics from port `9153` using the default path of `/metrics`.

```yaml
collectors:
  - name: "kube-dns"
    type: "endpoints"
    selectors:
      label: "kubernetes.io/name=KubeDNS"
    metric_port:
      value: "9153"
```

From all `pods` with a label `appName` set to `myapp`, collect if the pod has an annotation `metricsCollect` with a value of `true`, use the values of the `metricsPort` and `metricsPath` annotations when constructing the metric request URL. Add the static tag `env:prod` and use the item labels `appName` and `location` as tags.

```yaml
collectors:
  - name: "myapp"
    type: "pods"
    selectors:
      label: "appName=myapp"
    control:
      annotation: "metricsCollect"
      value: "true"
    metric_port:
      annotation: "metricsPort"
    metric_path:
      annotation: "metricsPath"
    tags: "env:prod"
    label_tags: "appName,location"
```
