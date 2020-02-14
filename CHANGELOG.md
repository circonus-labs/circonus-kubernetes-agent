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
