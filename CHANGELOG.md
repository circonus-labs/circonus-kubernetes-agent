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
