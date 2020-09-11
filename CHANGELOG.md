# v0.10.0

* doc: add observation deployment instructions
* add: observation/sizing mode deployment manifests `deploy/observation`
* upd: remove log warning when no metrics sent for specific collectors for sizing mode
* add: total events counter metric
* upd: rename internal tracking stats for clarity when logged
* add: ability to disable default alert rulesets
* upd: expose threshold & window settings for all default rulesets
* upd: refactor stream tag handling
* add: agent metric mutex
* upd: reset any observed event counters
* add: event counter metrics
* upd: validate tag category and value lengths to match broker rules
* add: max tag len, max category len
* add: timestamp to derived ksm metrics
* add: `collect_k8s_pod_count` metric
* add: `collect_k8s_node_count` metric
* fix: whitespace

# v0.9.10

* fix: separate ksm and agent metrics
* upd: refactor submission stats
* add: --log-agent-metrics for debugging

# v0.9.9

* upd: refactor, common clientset func
* add: k8s version metric `collect_k8s_ver`
* upd: switch to go-client for in-cluster api interactions
* upd: default k8s url host `kubernetes.default.svc`
* fix: downgrade to go1.14

# v0.9.8

* fix: revert back to `pod_status_ready` and `pod_status_scheduled`

# v0.9.7

* add: `pod_status` text metric filter
* upd: `pod_container_status` and `pod_status_phase` filters
* fix: lint min tls ver
* upd: go1.15

# v0.9.6

* add: `pod_status_phase` metric filter
* add: `pod_container_status` text metrics and metric filter

# v0.9.5

* add: derived metrics to enhance dashboard performance
* add: IncrementCounterByVvalue method
* upd: force lowercase tag categories
* upd: dependencies
* upd: stub hpa endpoints
* fix: binary name for updated goreleaser
* upd: refactor dns annotation and explicit port use

# v0.9.4

* add: kube-dns-metrics-port - used when scrape/port annotations are NOT defined on the kube-dns service (e.g. GKE)

# v0.9.3

* fix: broker cn check both ip and external_host
* upd: add debug line when using custom api ca  cert
* upd: use `latest` for default (simple) deployment
* upd: remove path validation from api url
* upd: add `/v2` path to default api url

# v0.9.2

* upd: explicit cases for prometheus metric types
* add: golangci-lint action
* add: ksm collection state metric
* upd: collect immediately then start intervals
* add: dns collection state metric
* fix: correct dns config option name
* doc: update wiht dns configuration information
* add: support `kubedns*` metrics if backing kube-dns service
* add: initial event if event watching is enabled
* add: support for broker's "filtered" back into  per submission stat
* add: logging of result if broker returned an err msg

# v0.9.1

* add: `lookup_key` to rule_sets
* upd: dependencies (apiclient, cgm, toml, yaml, viper, zerolog)

# v0.9.0

* upd: deployment configurations to v0.9.0
* add: ksm request mode (direct or proxy)
* mrg: ksm field selector update v0.8.0
* upd: use json for default rules, reduce friction of maintenance (both configuration.yaml and check.go use json)
* upd: metric filter rules for coredns health metrics
* add: default alerting and custom rules support
* add: `_avg` for prom histograms (sum/count) for health dashboard dns metrics
* add: config items for configmap json files (metric-filters.json, default-rules.json, custom-rules.json)
* upd: default settings enable required collections for dashbaord
* upd: split deployment configurations into two: `deploy/default/` (simplified) and `deploy/custom/` full control
* upd: dns metrics, use `pod` for tag
* upd: metric filters
* add: health dashboard specific metrics
* upd: remove nodeSelector from deployment.yaml (old k8s versions lack `kubernetes.io/os: linux` label)
* upd: configuration.yaml to enable needed collection to support dashboard
* add: metric filter rules for health dashboard
* add: node cpu utilization for health dashboard
* add: cluster name to check for tagging check, rules, contacts

# v0.8.0

* add: kube-state-metrics field selector

# v0.7.1

* add: contributed helm chart

# v0.7.0

* NOTE: metrics-server option is deprecated
* add: metric filter rules to configuration for dns, api errors, and api auth
* upd: refactor sequencing and number of go routines for metric collection
* add: basic local filtering of metrics by namerx in rules (reduce memory utilization, bandwidth, and broker load)
* add: `--nodecc` argument to turn on concurrent collection for node/pod/container metrics (mem vs speed/cpu tradeoff) default is off
* upd: collect dns metrics from each kube-dns pod, default true, for new health dashboard - can be turned off in configuration
* add: api-server metrics collection, default true, for new health dashboard - can be turned off in configuration

# v0.6.6

* fix: force float64 for used percentages
* upd: include `units:percent` for fs metric filters

# v0.6.5

* add: update metric filters from configuration on every start. deployment configuration is definitive source for metric filters.
* fix: add default tags to internal `collect_*` metrics
* fix: remove escaping quotes in string metric values
* upd: remove unused methods
* add: used percent for fs/volume metrics
* add: `metrics.k8s.io` to rbac

# v0.6.4

* add: support https for certain kube-state-metrics configurations. port names prefixed with `https-` will trigger apiserver proxy urls using `https:`.

# v0.6.3

* add: make kube-state-metrics port names for metrics and telemetry configurable. Default from ['standard' service deployment](https://github.com/kubernetes/kube-state-metrics/blob/master/examples/standard/service.yaml). metrics=`http-metrics` and telemetry=`telemetry`.

# v0.6.2

* add: optional, metric collection for kube-dns
* upd: default check.target to cluster.name if target is unset
* add: error if neither metric or telemetry ports found in ksm service definition
* add: warn if metric port not found in ksm service definition
* add: warn if telemetry port not found in ksm service definition
* add: debug message for ksm urls being used

# v0.6.1

* add: `__rollup:false` stream tag to remaining high cardinality metrics

# v0.6.0

* Switch to `httptrap:kubernetes` check type. To preserve metric continuity - if
an `httptrap:kubernetes` check is not found, the agent will search for an `httptrap`
check and use that if found. Otherwise, it will create a new check using the new
check with correct sub-type.

# v0.5.8

* add: optional collection of cadvisor metrics from kubelet

# v0.5.7

* add: option `--serial-submissions` to disable concurrent submissions
* upd: default to concurrent submissions w/timestamp metrics
* upd: deprecate streaming metrics as an option
* fix: use metric queue for events
* fix: ensure all metrics have timestamp, address drift on retries
* fix: `/health` output

# v0.5.6

* upd: sequential to use contextual logger for messages when submitting
* add: use sequential for stream if not concurrent
* add: status code to api response errors
* add: sequential metric submitter (gaps)
* add: option for concurrent metric submission
* add: option for max metric bucket size for promtext
* upd: use config option for max metric bucket size
* fix: remove redundant call parameter for promtext

# v0.5.5

* upd: increase metric bucket size to 1000
* fix: typo in metric name
* upd: use `resultLogger` to identify source of submission errors/retries
* add: set `collect_submit_retries` to 0 at start of each collection run

# v0.5.4

* add: liveness probe support `/health`
* upd: increase default pool size to 2
* upd: resource request/limit example (commented out)
* add: ksm service error collect metric

# v0.5.3

* fix: `async_metrics` (queue/stream)
* fix: drain streamed metrics in bucket
* fix: use cluster ctx to honor signals

# v0.5.2

* add: collect submit counters (success,fail,error)

# v0.5.1

* upd: normalize collection metric tags for more clarity

# v0.5.0

* add: logging on submit retries and non-200 responses
* add: api request timelimit (default:10s)
* add: `kube_pod_deleted` to metric filters
* add: agent metric
* add: api request error metrics
* add: collection duration, api latency, and metric submission latency histograms
* upd: failure message when cluster(s) can't be initialized resulting in 0 clusters
* fix: ensure default tags used in queued metrics

# v0.4.5

* fix: handle empty tag lists better, e.g. events with no tags
* doc: elaborate on need for settings to be uncommented in both configuration and deployment
* fix: remove `available` memory metric for pods and containers since it is not provided

# v0.4.4

* fix: typo in network errors rule

# v0.4.3

* upd: metric filter rule to include storage `capacity` for pod volumes
* add: per-interface network metrics
* add: node `capacity_ephemeral_storage` metric

# v0.4.2

* add: node capacity metrtics: `capacity_cpu`, `capacity_memory`, and `capacity_pods`

# v0.4.1

* add: `collect_interval` metric
* add: `default_streamtags` option to apply a set of tags to _all_ metrics

# v0.4.0

* upd: implement check `metric_filters` to collect only metrics in dashboard
* add: chunk large arbitrary metric collectors (ksm and ms)
* upd: collection metrics - remove accept/filter, add agent memory metrics
* add: collection metrics - agent memory and goroutine metrics
* upd: dependencies

# v0.3.1

* add: collection summary metrics (sent,accept,filter,bytes,duration)

# v0.3.0

* add: `--no-base64` switch to disable for test/debug (base64 stream tags)
* add: `--no-gzip` switch to disable for test/debug (gzip trap submissions)
* upd: switch to using gzip compression for trap submissions as default
* fix: stream tag quote escaping in printf

# v0.2.0

* add: abridged events
* add: pod filtering by label key/val

# v0.1.0

* initial preview release
