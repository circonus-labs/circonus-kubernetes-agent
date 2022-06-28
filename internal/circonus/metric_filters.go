// Copyright © 2021 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package circonus

import (
	"encoding/json"
	"io/ioutil"

	"github.com/circonus-labs/circonus-kubernetes-agent/internal/config/keys"

	"github.com/hashicorp/go-version"
	"github.com/spf13/viper"
)

const (
	defaultMetricFiltersStr117 = `
{
    "metric_filters": [
	["allow", "^.+$", "tags", "and(collector:dynamic)", "NO_LOCAL_FILTER dynamically collected metrics"],
    ["allow", "^kubelet_.*$", "node metrics k8s v1.18+"],
    ["allow", "^machine_.*$", "node metrics k8s v1.18+"],
    ["allow", "^(container|node|pod)_.*$", "node metrics k8s v1.18+"],
    ["allow", "^prober_.*$", "node metrics/probes k8s v1.18+"],
    ["allow", "^[rt]x$", "tags", "and(resource:network,or(units:bytes,units:errors),not(container_name:*),not(sys_container:*))", "utilization"],
    ["allow", "^(used|capacity)$", "tags", "and(or(units:bytes,units:percent),or(resource:memory,resource:fs,volume_name:*),not(container_name:*),not(sys_container:*))", "utilization"],
	["allow", "^usage(Milli|Nano)Cores$", "tags", "and(not(container_name:*),not(sys_container:*))", "utilization"],
	["allow", "^resource_(request|limit)$", "resources"],
    ["allow", "^apiserver_request_total$", "tags", "and(or(code:5*,code:4*))", "api req errors"],
    ["allow", "^authenticated_user_requests$", "api auth"],
    ["allow", "^(kube_)?pod_container_status_(running|terminated|waiting|ready)(_count)?$", "containers"],
    ["allow", "^pod_container_status$", "containers"],
    ["allow", "^(kube_)?pod_container_status_(terminated|waiting)_reason(_count)?$", "containers health"],
    ["allow", "^(kube_)?pod_init_container_status_(terminated|waiting)_reason(_count)?$", "containers health"],
    ["allow", "^kube_deployment_(created|spec_replicas)$", "deployments"],
    ["allow", "^kube_deployment_status_(replicas|replicas_updated|replicas_available|replicas_unavailable)$", "deployments"],
    ["allow", "^kube_job_status_failed$", "health"],
    ["allow", "^kube_persistentvolume_status_phase$", "health"],
	["allow", "^kube_deployment_status_replicas_unavailable$", "deployments"],
    ["allow", "^kube_hpa_(spec_max|status_current)_replicas$", "scale"],
    ["allow", "^kube_pod_start_time$", "pods"],
    ["allow", "^kube_pod_status_condition$", "pods"],
    ["allow", "^(kube_)?pod_status_phase(_count)?$", "tags", "and(or(phase:Running,phase:Pending,phase:Failed,phase:Succeeded))", "pods"],
    ["allow", "^pod_status_phase$", "pods"],
    ["allow", "^(kube_)?pod_status_(ready|scheduled)(_count)?$", "tags", "and(condition:true)", "pods"],
    ["allow", "^pod_status_(ready|scheduled)$", "pods"],
    ["allow", "^kube_(service_labels|deployment_labels|pod_container_info|pod_deleted)$", "ksm inventory"],
    ["allow", "^(node|kubelet_running_pod_count|Ready)$", "nodes"],
    ["allow", "^NetworkUnavailable$", "node status"],
    ["allow", "^kube_node_status_condition$", "node status health"],
    ["allow", "^(Disk|Memory|PID)Pressure$", "node status"],
    ["allow", "^capacity_.*$", "node capacity"],
    ["allow", "^kube_namespace_status_phase$", "tags", "and(or(phase:Active,phase:Terminating))", "namespaces"],
    ["allow", "^utilization$", "utilization health"],
    ["allow", "^kube_deployment_(metadata|status_observed)_generation$", "health"],
    ["allow", "^kube_daemonset_status_(current|desired)_number_scheduled$", "health"],
    ["allow", "^kube_statefulset_status_(replicas|replicas_ready)$", "health"],
    ["allow", "^deployment_generation_delta$", "health"],
    ["allow", "^daemonset_scheduled_delta$", "health"],
    ["allow", "^statefulset_replica_delta$", "health"],
    ["allow", "^coredns_(dns|forward)_request_(count_total|duration_seconds_avg)$", "dns health"],
    ["allow", "^coredns_(dns|forward)_response_rcode_count_total$", "dns health"],
    ["allow", "^kubedns*","dns health"],
    ["allow", "^events$", "events"],
    ["allow", "^collect_.*$", "agent collection stats"],
    ["allow", "^authentication_attempts$", "api auth health"],
    ["allow", "^cadvisor.*$", "cadvisor"],
    ["deny", "^.+$", "all other metrics"]
    ]
}
`
	defaultMetricFiltersStr118 = `
{
    "metric_filters": [
	["allow", "^.+$", "tags", "and(collector:dynamic)", "NO_LOCAL_FILTER dynamically collected metrics"],
    ["allow", "^(pod|node)_cpu_usage_seconds_total$", "utilization"],
    ["allow", "^(pod|node)_memory_working_set_bytes$", utilization"],
    ["allow", "^(kube_)?pod_container_status_(running|terminated|waiting|ready)(_count)?$", "containers"],
    ["allow", "^(kube_)?pod_container_status_(terminated|waiting)_reason(_count)?$", "containers health"],
    ["allow", "^(kube_)?pod_init_container_status_(terminated|waiting)_reason(_count)?$", "init containers health"],
    ["allow", "^kube_deployment_(created|spec_replicas)$", "deployments"],
    ["allow", "^kube_job_status_failed$", "health"],
    ["allow", "^kube_persistentvolume_status_phase$", "health"],
	["allow", "^kube_deployment_status_replicas_unavailable$", "deployments"],
    ["allow", "^kube_hpa_(spec_max|status_current)_replicas$", "scale"],
    ["allow", "^kube_pod_start_time$", "pods"],
    ["allow", "^(kube_)?pod_status_phase(_count)?$", "tags", "and(or(phase:Running,phase:Pending,phase:Failed,phase:Succeeded))", "pods"],
    ["allow", "^pod_status_phase$", "pods"],
    ["allow", "^kube_pod_info$", "pods"],
    ["allow", "^kube_(service|deployment)_labels$", "ksm inventory"],
    ["allow", "^kube_node_spec_unschedulable$", "node status"],
    ["allow", "^kube_node_status_allocatable$", "node status"],
    ["allow", "^kube_node_status_condition$", "node status health"],
    ["allow", "^kube_namespace_status_phase$", "tags", "namespaces"],
    ["allow", "^utilization$", "utilization health"],
    ["allow", "^kube_deployment_(metadata|status_observed)_generation$", "health"],
    ["allow", "^kube_daemonset_status_(current|desired)_number_scheduled$", "health"],
    ["allow", "^kube_statefulset_status_(replicas|replicas_ready)$", "health"],
    ["allow", "^deployment_generation_delta$", "health"],
    ["allow", "^daemonset_scheduled_delta$", "health"],
    ["allow", "^statefulset_replica_delta$", "health"],
    ["allow", "^coredns_(dns|forward)_request_(count_total|duration_seconds_avg)$", "dns health"],
    ["allow", "^coredns_(dns|forward)_response_rcode_count_total$", "dns health"],
    ["allow", "^kubedns*","dns health"],
    ["allow", "^events$", "events"],
    ["allow", "^collect_.*$", "agent collection stats"],
    ["allow", "^authentication_attempts$", "api auth health"],
    ["allow", "^cadvisor.*$", "cadvisor"],
    ["deny", "^.+$", "all other metrics"]
    ]
}
`
)

type metricFilters struct {
	Filters [][]string `json:"metric_filters"`
}

func (c *Check) loadMetricFilters() [][]string {
	mfConfigFile := viper.GetString(keys.MetricFiltersFile)
	data, err := ioutil.ReadFile(mfConfigFile)
	if err != nil {
		c.log.Warn().Err(err).Str("metric_filter_config", mfConfigFile).Msg("using defaults")
		return c.defaultFilters()
	}

	var mf metricFilters
	if err := json.Unmarshal(data, &mf); err != nil {
		c.log.Warn().Err(err).Str("metric_filter_config", mfConfigFile).Msg("using defaults")
		return c.defaultFilters()
	}

	return mf.Filters
}

func (c *Check) defaultFilters() [][]string {

	var defaultMetricFiltersData []byte

	currversion, err := version.NewVersion(c.clusterVers)
	if err != nil {
		c.log.Warn().Err(err).Msg("parsing api version")
		return [][]string{
			{"deny", "^$", "empty"},
			{"allow", "^.+$", "all"},
		}
	}

	v118, err := version.NewVersion("v1.18")
	if err != nil {
		c.log.Warn().Err(err).Msg("parsing v1.18")
		return [][]string{
			{"deny", "^$", "empty"},
			{"allow", "^.+$", "all"},
		}
	}

	if currversion.LessThan(v118) {
		defaultMetricFiltersData = []byte(defaultMetricFiltersStr117)
	} else {
		defaultMetricFiltersData = []byte(defaultMetricFiltersStr118)
	}

	var mf metricFilters
	if err := json.Unmarshal(defaultMetricFiltersData, &mf); err != nil {
		c.log.Warn().Err(err).Msg("parsing default metric filters")
		return [][]string{
			{"deny", "^$", "empty"},
			{"allow", "^.+$", "all"},
		}
	}
	return mf.Filters
}
