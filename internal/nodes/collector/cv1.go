// Copyright © 2021 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package collector

import (
	"encoding/json"
	"time"

	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/circonus"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/k8s"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/release"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// collector v1 methods (for k8s < v1.20)

// summary emits node summary stats
func (nc *Collector) summary(parentStreamTags []string, parentMeasurementTags []string) {
	if nc.done() {
		return
	}

	start := time.Now()

	clientset, err := k8s.GetClient(&nc.cfg)
	if err != nil {
		nc.log.Error().Err(err).Msg("initializing client set for stats/summary, abandoning collection")
		return
	}

	req := clientset.CoreV1().RESTClient().Get().RequestURI(nc.baseURI + "/proxy/stats/summary")
	res := req.Do(nc.ctx)
	data, err := res.Raw()
	if err != nil {
		nc.check.IncrementCounter("collect_api_errors", cgm.Tags{
			cgm.Tag{Category: "source", Value: release.NAME},
			cgm.Tag{Category: "request", Value: "stats/summary"},
			cgm.Tag{Category: "proxy", Value: "api-server"},
			cgm.Tag{Category: "target", Value: "kubelet"},
		})
		nc.log.Error().Err(err).Str("url", req.URL().String()).Msg("fetching stats/summary stats")
		return
	}

	nc.check.AddHistSample("collect_latency", cgm.Tags{
		cgm.Tag{Category: "source", Value: release.NAME},
		cgm.Tag{Category: "request", Value: "stats/summary"},
		cgm.Tag{Category: "proxy", Value: "api-server"},
		cgm.Tag{Category: "target", Value: "kubelet"},
		cgm.Tag{Category: "units", Value: "milliseconds"},
	}, float64(time.Since(start).Milliseconds()))

	var stats statsSummary
	if err := json.Unmarshal(data, &stats); err != nil {
		nc.log.Error().Err(err).Msg("parsing stats/summary metrics")
		return
	}

	nc.check.AddGauge("collect_k8s_pod_count", cgm.Tags{
		cgm.Tag{Category: "node", Value: nc.node.Name},
		cgm.Tag{Category: "source", Value: release.NAME},
	}, len(stats.Pods))

	nc.summaryNode(&stats.Node, parentStreamTags, parentMeasurementTags)
	nc.summarySystemContainers(&stats.Node, parentStreamTags, parentMeasurementTags)
	nc.summaryPods(&stats, parentStreamTags, parentMeasurementTags)
}

func (nc *Collector) summaryNode(node *statsSummaryNode, parentStreamTags []string, parentMeasurementTags []string) {
	if nc.done() {
		return
	}

	metrics := make(map[string]circonus.MetricSample)

	nc.queueCPU(metrics, &node.CPU, true, parentStreamTags, parentMeasurementTags)
	nc.queueMemory(metrics, &node.Memory, parentStreamTags, parentMeasurementTags, true)
	nc.queueNetwork(metrics, &node.Network, parentStreamTags, parentMeasurementTags)
	nc.queueFS(metrics, &node.FS, parentStreamTags, parentMeasurementTags)
	nc.queueRuntimeImageFS(metrics, &node.RuntimeFS.ImageFs, parentStreamTags, parentMeasurementTags)
	nc.queueRlimit(metrics, &node.Rlimit, parentStreamTags, parentMeasurementTags)

	if len(metrics) == 0 {
		// nc.log.Warn().Msg("no summary telemetry to submit")
		return
	}
	if err := nc.check.SubmitMetrics(nc.ctx, metrics, nc.log.With().Str("type", "/stats/summary").Logger(), true); err != nil {
		nc.log.Warn().Err(err).Msg("submitting metrics")
	}
}

func (nc *Collector) summarySystemContainers(node *statsSummaryNode, parentStreamTags []string, parentMeasurementTags []string) {
	if nc.done() {
		return
	}
	if !nc.cfg.IncludeContainers {
		return
	}
	if len(node.SystemContainers) == 0 {
		nc.log.Error().Msg("invalid system containers (none)")
		return
	}

	metrics := make(map[string]circonus.MetricSample)

	for _, container := range node.SystemContainers {
		if nc.done() {
			break
		}
		streamTags := nc.check.NewTagList(parentStreamTags, []string{"sys_container:" + container.Name})
		nc.queueCPU(metrics, &container.CPU, false, streamTags, parentMeasurementTags)
		nc.queueMemory(metrics, &container.Memory, streamTags, parentMeasurementTags, false)
		nc.queueRootFS(metrics, &container.RootFS, streamTags, parentMeasurementTags)
		nc.queueLogsFS(metrics, &container.Logs, streamTags, parentMeasurementTags)
	}

	if len(metrics) == 0 {
		// nc.log.Warn().Msg("no system container telemetry to submit")
		return
	}
	if err := nc.check.SubmitMetrics(nc.ctx, metrics, nc.log.With().Str("type", "system_containers").Logger(), true); err != nil {
		nc.log.Warn().Err(err).Msg("submitting metrics")
	}
}

func (nc *Collector) summaryPods(stats *statsSummary, parentStreamTags []string, parentMeasurementTags []string) {
	if nc.done() {
		return
	}
	if !nc.cfg.IncludePods {
		return
	}
	if len(stats.Pods) == 0 {
		nc.log.Error().Msg("invalid pods (none)")
		return
	}

	metrics := make(map[string]circonus.MetricSample)

	// clientset
	clientset, err := k8s.GetClient(&nc.cfg)
	if err != nil {
		nc.log.Error().Err(err).Msg("initializing client set for pods, abandoning collection")
		return
	}

	for _, pod := range stats.Pods {
		if nc.done() {
			break
		}
		podSpec, err := clientset.CoreV1().Pods(pod.PodRef.Namespace).Get(nc.ctx, pod.PodRef.Name, metav1.GetOptions{})
		if err != nil {
			nc.check.IncrementCounter("collect_api_errors", cgm.Tags{
				cgm.Tag{Category: "source", Value: release.NAME},
				cgm.Tag{Category: "request", Value: "pod"},
				cgm.Tag{Category: "target", Value: "api-server"},
			})
			nc.log.Error().Err(err).Str("pod", pod.PodRef.Name).Str("ns", pod.PodRef.Namespace).Msg("fetching pod, skipping")
			continue
		}

		collect, podLabels := nc.getPodLabels(podSpec)
		if !collect {
			continue
		}

		podStreamTags := nc.check.NewTagList(parentStreamTags, []string{
			"pod:" + pod.PodRef.Name,
			"namespace:" + pod.PodRef.Namespace,
			"__rollup:false", // prevent high cardinality metrics from rolling up
		}, podLabels)

		nc.queueCPU(metrics, &pod.CPU, false, podStreamTags, parentMeasurementTags)
		nc.queueMemory(metrics, &pod.Memory, podStreamTags, parentMeasurementTags, false)
		nc.queueNetwork(metrics, &pod.Network, podStreamTags, parentMeasurementTags)

		totUsed := uint64(0)
		for _, volume := range pod.Volumes {
			if nc.done() {
				break
			}
			volume := volume
			nc.queueVolume(metrics, &volume, podStreamTags, parentMeasurementTags)
			totUsed += volume.UsedBytes
		}

		nc.queueEphemeralStorage(metrics, &pod.EphemeralStorage, podStreamTags, parentMeasurementTags)
		totUsed += pod.EphemeralStorage.UsedBytes

		{ // total used volume and ephemeral-storage bytes for pod
			streamTags := nc.check.NewTagList(podStreamTags, []string{"units:bytes", "resource:fs"})
			_ = nc.check.QueueMetricSample(metrics, "used", circonus.MetricTypeUint64, streamTags, parentMeasurementTags, totUsed, nc.ts)
		}

		nc.resourceMetrics(metrics, podSpec.Spec, podStreamTags, parentMeasurementTags)

		if nc.cfg.IncludeContainers {
			for _, container := range pod.Containers {
				if nc.done() {
					break
				}
				streamTagList := nc.check.NewTagList(podStreamTags, []string{"container_name:" + container.Name})
				nc.queueCPU(metrics, &container.CPU, false, streamTagList, parentMeasurementTags)
				nc.queueMemory(metrics, &container.Memory, streamTagList, parentMeasurementTags, false)
				if container.RootFS.CapacityBytes > 0 { // rootfs
					nc.queueRootFS(metrics, &container.RootFS, streamTagList, parentMeasurementTags)
				}
				if container.Logs.CapacityBytes > 0 { // logs
					nc.queueLogsFS(metrics, &container.Logs, streamTagList, parentMeasurementTags)
				}
			}
		}
	}

	if len(metrics) == 0 {
		// nc.log.Warn().Msg("no pod telemetry to submit")
		return
	}
	if err := nc.check.SubmitMetrics(nc.ctx, metrics, nc.log.With().Str("type", "pods").Logger(), true); err != nil {
		nc.log.Warn().Err(err).Msg("submitting metrics")
	}
}
