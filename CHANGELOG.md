# **unreleased**

## v0.19.2

* build(deps): bump github.com/spf13/viper from 1.18.1 to 1.18.2
* build(deps): bump github/codeql-action from 2 to 3
* build(deps): bump github.com/spf13/viper from 1.18.0 to 1.18.1
* build(deps): bump github.com/spf13/viper from 1.17.0 to 1.18.0
* chore: remove codeql workflow as nobody uses it
* fix: config test with no config, should get error
* build(deps): bump github.com/circonus-labs/go-apiclient from 0.7.23 to 0.7.24
* build: add changelog config
* build: skip go in lint workflow

## v0.19.1

* build: add after hook for `grype` on generated sboms
* build: add .sbom for archive artifacts
* build: add before hooks for `go mod tidy`, `govulncheck` and `golangci-lint`

## v0.19.0

* chore(goreleaser): remove archives.rlcp -- deprecated
* fix: binary location for copy (docker)
* fix: changelog typos and h1>h2
* build(deps): bump github.com/spf13/cobra from 1.6.1 to 1.8.0
* build(deps): bump github.com/circonus-labs/circonus-gometrics/v3 from 3.4.6 to 3.4.7
* build(deps): bump github.com/spf13/viper from 1.15.0 to 1.17.0
* fix: vulnerability golang.org/x/net to v0.17.0
* build(deps): bump github.com/rs/zerolog from 1.29.0 to 1.31.0
* fix(cv1): more implicit memory usage in loop
* fix(cv1): implicit memory access
* build(deps): bump go.uber.org/automaxprocs from 1.5.2 to 1.5.3

## v0.18.0

* feat: add interval, collect deadline, submit deadline to collect config startup message
* feat: add uncompressed data size to stats
* fix: clarify tracking messages
* fix: typo in key name
* feat: add kubelet ver and ver mode info msg
* chore: add rlcp:true for future deprecation
* chore: debug msgs for deadlines
* feat: add --submit-deadline option
* feat: add --collect-deadline option
* chore: use submit and collect deadline options
* chore: switch from submit metrics to flush collector metrics wrapper
* chore: improved stats messaging
* fix: make metric submitter private and add public wrapper with deadline

## v0.17.0

* fix(goreleaser): deprecated syntax
* build(deps): bump github.com/circonus-labs/go-apiclient from 0.7.22 to 0.7.23
* feat: implement 30s deadline with retry on metric submission
* fix: update deps for security vulnerabilities
* fix: tags for submission metrics

## v0.16.1

* feat: add start/finish status messages around collectors

## v0.16.0

* feat: add `summary/stats` into v2 collection (was supposed to have been deprecated in 1.18) but is still present and user is looking for `usageNanoCores`.

## v0.15.0

* chore: update warning when alert config file not found to not stutter the file nam
* fix: clean non-semver GKE k8s version (metric filters)
* feat: update skydns filter to include all metrics
* chore: update warning when metric filter file not found to not stutter the file name
* feat: debug messages for using config vs annotations
* feat: emit collection config info msg
* fix: struct alignment
* fix: check K8SEnableDNSMetrics if scrape is false
* fix: port message when using prot from config instead of annotation
* fix: skip events older than 1m
* fix: clean non-semver GKE k8s version (node collectors)
* chore: clean up imports
* fix: add GKE skydns filter
* fix: drop NaN metrics when queueing
* feat: go1.19 for strings.Cut

## v0.14.0

* feat: add node_name tag for pods
* feat: add owner tag for pods
* feat: add support for label_tags=* to turn all labels into tags for the given object

## v0.13.0

2022-08-08

([833c904b](https://github.com/circonus-labs/circonus-kubernetes-agent/commit/833c904b))  
            Tags: v0.13.0

([ea5236bb](https://github.com/circonus-labs/circonus-kubernetes-agent/commit/ea5236bb))  
            Add support for CoreDNS.
            Add auto-detection of CoreDNS if detection of kube-dns fails.
            Add configuration key to enable users to set the default CoreDNS
            metrics scrape port.

([34650875](https://github.com/circonus-labs/circonus-kubernetes-agent/commit/34650875))  
            Fix agent trying to add an erroneous tag to the check bundle.

([3c1bb387](https://github.com/circonus-labs/circonus-kubernetes-agent/commit/3c1bb387))  
            Revert change to move metric creation into conditional in cluster
            collector.

([328f105d](https://github.com/circonus-labs/circonus-kubernetes-agent/commit/328f105d))  
            Fix logic in creating default rulesets and metric filters to return
            required vals and determine cluster version before creating them.

([36652400](https://github.com/circonus-labs/circonus-kubernetes-agent/commit/36652400))  
            Refactor code to be more easily readable and and check all errors.

([19fbcc25](https://github.com/circonus-labs/circonus-kubernetes-agent/commit/19fbcc25))  
            Fix whitespace issues in self-contained metricfilters.
            Fix error wrapping in k8s api call.
            Add k8s version const for comparison usage.

([3a0637ae](https://github.com/circonus-labs/circonus-kubernetes-agent/commit/3a0637ae))  
            Fix default metric filters for k8s v1.20+ to search for the correct
            metric names.

([4f532ab5](https://github.com/circonus-labs/circonus-kubernetes-agent/commit/4f532ab5))  
            This update adds support for CoreDNS, which is automatically detected
            if kube-dns does not exist.
            It also adds a configuration key to enable users to set the default
            CoreDNS metrics scrape port.
            We can expect that users will run kube-dns OR coredns, but not both.

([5576f42e](https://github.com/circonus-labs/circonus-kubernetes-agent/commit/5576f42e))  
            This change will resolve the agent trying to add an erroneous tag to
            the check bundle.

([619f9973](https://github.com/circonus-labs/circonus-kubernetes-agent/commit/619f9973))  
            This just moves a call to AddText out of a conditional

([6820aa34](https://github.com/circonus-labs/circonus-kubernetes-agent/commit/6820aa34))  
            This change will fix the logic to return the values required and
            correctly determine the cluster version for creating the default rules
            and filters.

([70216e56](https://github.com/circonus-labs/circonus-kubernetes-agent/commit/70216e56))  
            This change refactors code to be simpler (more easily readable) and
            check all errors

([8b8935a8](https://github.com/circonus-labs/circonus-kubernetes-agent/commit/8b8935a8))  
            This change is mostly for code quality reasons and will likely have
            minimal visibility to our users (aside from the ones who decide to
            audit the k8s agent).

([af4bcb01](https://github.com/circonus-labs/circonus-kubernetes-agent/commit/af4bcb01))  
            This change will fix the v1.20+ MetricFilters to search for the correct
            metric names.

([1d64029c](https://github.com/circonus-labs/circonus-kubernetes-agent/commit/1d64029c))  
            dependency-name: github.com/spf13/cobra
              dependency-type: direct:production
              update-type: version-update:semver-minor

([f7aa1308](https://github.com/circonus-labs/circonus-kubernetes-agent/commit/f7aa1308))  
            dependency-name: actions/checkout
              dependency-type: direct:production
              update-type: version-update:semver-major

## v0.12.6

* upd: log error message on cn mismatch in tls verify
* upd: lint issues
* upd: add pre-build lint back in
* add: build_lint.sh script
* upd: lint config
* fix: ensure probe and resource metrics collected in sequential mode
* build(deps): bump github.com/spf13/viper from 1.10.0 to 1.10.1
* build(deps): bump github.com/rs/zerolog from 1.26.0 to 1.26.1

## v0.12.5

* upd: update dependencies to latest versions

## v0.12.4

* add: namespace tag to all dc targets

## v0.12.3

* add: metric_filter configuration to helm chart files
* build(deps): bump github.com/spf13/viper from 1.8.1 to 1.9.0

## v0.12.2

* upd: bump github.com/rs/zerolog from 1.24.0 to 1.25.0
* upd: disable rule sets by default
* upd: cgm v3.4.6
* upd: ignore test/
* upd: tighten tag limit enforcement
* upd: struct align
* build(deps): bump github.com/rs/zerolog from 1.23.0 to 1.24.0
* build(deps): bump github.com/pelletier/go-toml from 1.9.3 to 1.9.4

## v0.12.1

* fix: broker cluster support for CA/CN validation
* upd: struct align
* upd: contributed helm chart

## v0.12.0

* add: configuration options for >v1.18 k8s node metrics
* add: support v1.18+ deprecation of cadvisor endpoints
* upd: min tls ver
* upd: lint struct size
* upd: dependencies
* add: dependabot
* upd: lint ver
* upd: dependencies (viper/cobra/zerolog/etc.)

## v0.11.7

* upd: add metric type to nan detection err msg
* upd: dc request timeouts
* add: `collect_deadline_timeout` tracking metric
* add: collection deadline tied to collection interval
* upd: dependencies
* add: automaxprocs
* upd: syntax change for docker
* upd: lint version 1.38
* mrg: PR59 - custom/deployment.yaml mismatch with args_kubernetes.go

## v0.11.6

* upd: enable cumulative histogram support
* upd: change [dockerhub organization](https://hub.docker.com/repository/docker/circonus/circonus-kubernetes-agent) circonuslabs->circonus

## v0.11.5

* doc: update dynamic collector documentation
* upd: centralize config parsing
* upd: control setting, use label/annotation value as a boolean rather than comparison
* add: more logging on setting parse failures
* add: node/pod status skip if not ready

## v0.11.4

* add: dynamic collector filter to allow all dc metrics by default
* add: only apply local filters if rule enabled
* add: enable flag on filter rules
* add: disable dynamic collector rule with no local filter
* upd: dependencies (specifically cgm for better invalid tag messages)
* upd: pass cluster config to check
* fix: rollup setting parsing
* fix: filter and log NaNs

## v0.11.3

* fix: default dynamic collector file extension .json->.yaml (must be yaml)

## v0.11.2

* add: support annotation/label/value for dynamic collection rollup

## v0.11.1

* add: rollup setting for dynamic collection
* add: support annotation/label/value for schema

## v0.11.0

* add: dynamic collectors - define objects (endpoints, nodes, pods, services) to collect metrics from in configuration CIRC-5871
* upd: refactor ksm collection to be more intelligent w/re to port used (not all deployment methods name ports the same) CIRC-5890
* add: static ksm port option to configuration CIRC-5890
* upd: deprecate ksm mode and telemetry port options CIRC-5890
* upd: `pod_pending`, `network_unavailable` and `cpu_utilization` rulesets with `windowing_min_duration` CIRC-5875
* upd: return error when no metrics received from ksm so it can be expose in dashboard
* upd: add check for NaN values (skip) in metric processing
* upd: emit warning when no metrics to submit, with number processed (e.g. locally filtered)
* upd: use epoch for log timestamps (performance)
* upd: refactor cli arg handling

## v0.10.4

* add: additional logging for ksm collection/processing
* upd: add field selector to ksm errors for service and endpoint queries
* upd: example args (debug) to default deployment
* upd: switching to main errors pkg and new error handling
* upd: latest lint release

## v0.10.3

* upd: alter default `network_unavailable` ruleset to help with spurious alerts CIRC-5849

## v0.10.2

* upd: add on absence rule to all default rulesets in order to clear stale alerts
* upd: remove previously created ruleset if configuration updated to disable a default ruleset

## v0.10.1

* upd: send node conditions when status changes
* upd: remove unused text metrics
* add: usage millicores for res req/lim comparison
* fix: need ':' when category only tag
* upd: ensure sorted tag list
* add: resource request/limit metric filters
* add: hpa metric filters
* upd: add rollup:false to events

## v0.10.0

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

## v0.9.10

* fix: separate ksm and agent metrics
* upd: refactor submission stats
* add: --log-agent-metrics for debugging

## v0.9.9

* upd: refactor, common clientset func
* add: k8s version metric `collect_k8s_ver`
* upd: switch to go-client for in-cluster api interactions
* upd: default k8s url host `kubernetes.default.svc`
* fix: downgrade to go1.14

## v0.9.8

* fix: revert back to `pod_status_ready` and `pod_status_scheduled`

## v0.9.7

* add: `pod_status` text metric filter
* upd: `pod_container_status` and `pod_status_phase` filters
* fix: lint min tls ver
* upd: go1.15

## v0.9.6

* add: `pod_status_phase` metric filter
* add: `pod_container_status` text metrics and metric filter

## v0.9.5

* add: derived metrics to enhance dashboard performance
* add: IncrementCounterByValue method
* upd: force lowercase tag categories
* upd: dependencies
* upd: stub hpa endpoints
* fix: binary name for updated goreleaser
* upd: refactor dns annotation and explicit port use

## v0.9.4

* add: kube-dns-metrics-port - used when scrape/port annotations are NOT defined on the kube-dns service (e.g. GKE)

## v0.9.3

* fix: broker cn check both ip and external_host
* upd: add debug line when using custom api ca  cert
* upd: use `latest` for default (simple) deployment
* upd: remove path validation from api url
* upd: add `/v2` path to default api url

## v0.9.2

* upd: explicit cases for prometheus metric types
* add: golangci-lint action
* add: ksm collection state metric
* upd: collect immediately then start intervals
* add: dns collection state metric
* fix: correct dns config option name
* doc: update with dns configuration information
* add: support `kubedns*` metrics if backing kube-dns service
* add: initial event if event watching is enabled
* add: support for broker's "filtered" back into  per submission stat
* add: logging of result if broker returned an err msg

## v0.9.1

* add: `lookup_key` to rule_sets
* upd: dependencies (apiclient, cgm, toml, yaml, viper, zerolog)

## v0.9.0

* upd: deployment configurations to v0.9.0
* add: ksm request mode (direct or proxy)
* mrg: ksm field selector update v0.8.0
* upd: use json for default rules, reduce friction of maintenance (both configuration.yaml and check.go use json)
* upd: metric filter rules for coredns health metrics
* add: default alerting and custom rules support
* add: `_avg` for prom histograms (sum/count) for health dashboard dns metrics
* add: config items for configmap json files (metric-filters.json, default-rules.json, custom-rules.json)
* upd: default settings enable required collections for dashboard
* upd: split deployment configurations into two: `deploy/default/` (simplified) and `deploy/custom/` full control
* upd: dns metrics, use `pod` for tag
* upd: metric filters
* add: health dashboard specific metrics
* upd: remove nodeSelector from deployment.yaml (old k8s versions lack `kubernetes.io/os: linux` label)
* upd: configuration.yaml to enable needed collection to support dashboard
* add: metric filter rules for health dashboard
* add: node cpu utilization for health dashboard
* add: cluster name to check for tagging check, rules, contacts

## v0.8.0

* add: kube-state-metrics field selector

## v0.7.1

* add: contributed helm chart

## v0.7.0

* NOTE: metrics-server option is deprecated
* add: metric filter rules to configuration for dns, api errors, and api auth
* upd: refactor sequencing and number of go routines for metric collection
* add: basic local filtering of metrics by namerx in rules (reduce memory utilization, bandwidth, and broker load)
* add: `--nodecc` argument to turn on concurrent collection for node/pod/container metrics (mem vs speed/cpu tradeoff) default is off
* upd: collect dns metrics from each kube-dns pod, default true, for new health dashboard - can be turned off in configuration
* add: api-server metrics collection, default true, for new health dashboard - can be turned off in configuration

## v0.6.6

* fix: force float64 for used percentages
* upd: include `units:percent` for fs metric filters

## v0.6.5

* add: update metric filters from configuration on every start. deployment configuration is definitive source for metric filters.
* fix: add default tags to internal `collect_*` metrics
* fix: remove escaping quotes in string metric values
* upd: remove unused methods
* add: used percent for fs/volume metrics
* add: `metrics.k8s.io` to rbac

## v0.6.4

* add: support https for certain kube-state-metrics configurations. port names prefixed with `https-` will trigger api server proxy urls using `https:`.

## v0.6.3

* add: make kube-state-metrics port names for metrics and telemetry configurable. Default from ['standard' service deployment](https://github.com/kubernetes/kube-state-metrics/blob/master/examples/standard/service.yaml). metrics=`http-metrics` and telemetry=`telemetry`.

## v0.6.2

* add: optional, metric collection for kube-dns
* upd: default check.target to cluster.name if target is unset
* add: error if neither metric or telemetry ports found in ksm service definition
* add: warn if metric port not found in ksm service definition
* add: warn if telemetry port not found in ksm service definition
* add: debug message for ksm urls being used

## v0.6.1

* add: `__rollup:false` stream tag to remaining high cardinality metrics

## v0.6.0

* Switch to `httptrap:kubernetes` check type. To preserve metric continuity - if
an `httptrap:kubernetes` check is not found, the agent will search for an `httptrap`
check and use that if found. Otherwise, it will create a new check using the new
check with correct sub-type.

## v0.5.8

* add: optional collection of cadvisor metrics from kubelet

## v0.5.7

* add: option `--serial-submissions` to disable concurrent submissions
* upd: default to concurrent submissions w/timestamp metrics
* upd: deprecate streaming metrics as an option
* fix: use metric queue for events
* fix: ensure all metrics have timestamp, address drift on retries
* fix: `/health` output

## v0.5.6

* upd: sequential to use contextual logger for messages when submitting
* add: use sequential for stream if not concurrent
* add: status code to api response errors
* add: sequential metric submitter (gaps)
* add: option for concurrent metric submission
* add: option for max metric bucket size for promtext
* upd: use config option for max metric bucket size
* fix: remove redundant call parameter for promtext

## v0.5.5

* upd: increase metric bucket size to 1000
* fix: typo in metric name
* upd: use `resultLogger` to identify source of submission errors/retries
* add: set `collect_submit_retries` to 0 at start of each collection run

## v0.5.4

* add: liveness probe support `/health`
* upd: increase default pool size to 2
* upd: resource request/limit example (commented out)
* add: ksm service error collect metric

## v0.5.3

* fix: `async_metrics` (queue/stream)
* fix: drain streamed metrics in bucket
* fix: use cluster ctx to honor signals

## v0.5.2

* add: collect submit counters (success,fail,error)

## v0.5.1

* upd: normalize collection metric tags for more clarity

## v0.5.0

* add: logging on submit retries and non-200 responses
* add: api request time limit (default:10s)
* add: `kube_pod_deleted` to metric filters
* add: agent metric
* add: api request error metrics
* add: collection duration, api latency, and metric submission latency histograms
* upd: failure message when cluster(s) can't be initialized resulting in 0 clusters
* fix: ensure default tags used in queued metrics

## v0.4.5

* fix: handle empty tag lists better, e.g. events with no tags
* doc: elaborate on need for settings to be uncommented in both configuration and deployment
* fix: remove `available` memory metric for pods and containers since it is not provided

## v0.4.4

* fix: typo in network errors rule

## v0.4.3

* upd: metric filter rule to include storage `capacity` for pod volumes
* add: per-interface network metrics
* add: node `capacity_ephemeral_storage` metric

## v0.4.2

* add: node capacity metrics: `capacity_cpu`, `capacity_memory`, and `capacity_pods`

## v0.4.1

* add: `collect_interval` metric
* add: `default_streamtags` option to apply a set of tags to _all_ metrics

## v0.4.0

* upd: implement check `metric_filters` to collect only metrics in dashboard
* add: chunk large arbitrary metric collectors (ksm and ms)
* upd: collection metrics - remove accept/filter, add agent memory metrics
* add: collection metrics - agent memory and goroutine metrics
* upd: dependencies

## v0.3.1

* add: collection summary metrics (sent,accept,filter,bytes,duration)

## v0.3.0

* add: `--no-base64` switch to disable for test/debug (base64 stream tags)
* add: `--no-gzip` switch to disable for test/debug (gzip trap submissions)
* upd: switch to using gzip compression for trap submissions as default
* fix: stream tag quote escaping in printf

## v0.2.0

* add: abridged events
* add: pod filtering by label key/val

## v0.1.0

* initial preview release
