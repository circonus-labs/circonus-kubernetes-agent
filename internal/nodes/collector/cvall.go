// Copyright © 2021 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package collector

import (
	"bytes"
	"time"

	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/circonus"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/k8s"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/promtext"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/release"
	"github.com/prometheus/common/expfmt"
	"k8s.io/apimachinery/pkg/api/resource"
)

// collector common methods (for all k8s versions)

// meta emits node meta stats
func (nc *Collector) meta(parentStreamTags []string, parentMeasurementTags []string) {
	if nc.done() {
		return
	}

	start := time.Now()
	logger := nc.log.With().Str("type", "meta").Logger()
	logger.Debug().Msg("start")

	metrics := make(map[string]circonus.MetricSample)

	{ // meta
		streamTags := nc.check.NewTagList(parentStreamTags, []string{
			"kernel_version:" + nc.node.Status.NodeInfo.KernelVersion,
			"os_image:" + nc.node.Status.NodeInfo.OSImage,
			"kublet_version:" + nc.node.Status.NodeInfo.KubeletVersion,
		}, labelsToTags(nc.node.Labels))

		_ = nc.check.QueueMetricSample(
			metrics,
			"node",
			circonus.MetricTypeInt32,
			streamTags, parentMeasurementTags,
			1,
			nc.ts)
	}

	{ // conditions
		streamTags := nc.check.NewTagList(parentStreamTags, []string{"status:condition"})
		ns, found := GetNodeStat(nc.node.Name) // cache conditions and only emit when changed?
		if !found {
			ns = NewNodeStat()
		}
		for _, cond := range nc.node.Status.Conditions {
			if nc.done() {
				break
			}
			emit := true
			if lastCondStatus, ok := ns.Conditions[cond.Type]; ok {
				if lastCondStatus == cond.Status {
					emit = false
				}
			}
			ns.Conditions[cond.Type] = cond.Status
			if emit {
				_ = nc.check.QueueMetricSample(
					metrics,
					string(cond.Type),
					circonus.MetricTypeString,
					streamTags, parentMeasurementTags,
					cond.Message,
					nc.ts)
			}
		}
		SetNodeStat(nc.node.Name, ns)
	}

	{ // capacity and allocatable
		streamTags := nc.check.NewTagList(parentStreamTags)
		if qty, err := resource.ParseQuantity(nc.node.Status.Capacity.Cpu().String()); err == nil {
			if cpu, ok := qty.AsInt64(); ok {
				_ = nc.check.QueueMetricSample(
					metrics,
					"capacity_cpu",
					circonus.MetricTypeUint64,
					streamTags, parentMeasurementTags,
					uint64(cpu),
					nc.ts)
				ns, ok := GetNodeStat(nc.node.Name)
				if !ok {
					ns = NewNodeStat()
				}
				ns.CPUCapacity = uint64(cpu)
				SetNodeStat(nc.node.Name, ns)
			}
		} else {
			logger.Warn().Err(err).Str("cpu", nc.node.Status.Capacity.Cpu().String()).Msg("converting capacity.cpu")
			ns, ok := GetNodeStat(nc.node.Name)
			if !ok {
				ns = NewNodeStat()
				ns.CPUCapacity = uint64(1) // use 1 as a default placeholder
				SetNodeStat(nc.node.Name, ns)
			}
		}

		if qty, err := resource.ParseQuantity(nc.node.Status.Capacity.Pods().String()); err == nil {
			if pods, ok := qty.AsInt64(); ok {
				_ = nc.check.QueueMetricSample(
					metrics,
					"capacity_pods",
					circonus.MetricTypeUint64,
					streamTags, parentMeasurementTags,
					uint64(pods),
					nc.ts)
			}
		} else {
			logger.Warn().Err(err).Str("pods", nc.node.Status.Capacity.Pods().String()).Msg("converting capacity.pods")
		}

		if qty, err := resource.ParseQuantity(nc.node.Status.Capacity.Memory().String()); err == nil {
			if mem, ok := qty.AsInt64(); ok {
				sTags := nc.check.NewTagList(streamTags, []string{"units:bytes"})
				_ = nc.check.QueueMetricSample(
					metrics,
					"capacity_memory",
					circonus.MetricTypeUint64,
					sTags, parentMeasurementTags,
					uint64(mem),
					nc.ts)
			}
		} else {
			logger.Warn().Err(err).Str("memory", nc.node.Status.Capacity.Memory().String()).Msg("parsing quantity capacity.memory")
		}

		if qty, err := resource.ParseQuantity(nc.node.Status.Capacity.StorageEphemeral().String()); err == nil {
			if storage, ok := qty.AsInt64(); ok {
				sTags := nc.check.NewTagList(streamTags, []string{"units:bytes"})
				_ = nc.check.QueueMetricSample(
					metrics,
					"capacity_ephemeral_storage",
					circonus.MetricTypeUint64,
					sTags, parentMeasurementTags,
					uint64(storage),
					nc.ts)
			}
		} else {
			logger.Warn().Err(err).Str("ephemeral_storage", nc.node.Status.Capacity.StorageEphemeral().String()).Msg("parsing quantity capacity.ephemeral-storage")
		}
	}

	if len(metrics) == 0 {
		logger.Warn().Str("duration", time.Since(start).String()).Msg("no meta telemetry to submit")
		return
	}
	if err := nc.check.FlushCollectorMetrics(nc.ctx, metrics, logger, true); err != nil {
		logger.Warn().Err(err).Msg("submitting metrics")
	}
	logger.Debug().Str("duration", time.Since(start).String()).Msg("complete")
}

// nmetrics emits metrics from the node /metrics endpoint
func (nc *Collector) nmetrics(parentStreamTags []string, parentMeasurementTags []string) {
	if nc.done() {
		return
	}

	start := time.Now()
	logger := nc.log.With().Str("type", "/metrics").Logger()
	logger.Debug().Msg("start")

	clientset, err := k8s.GetClient(&nc.cfg)
	if err != nil {
		logger.Error().Err(err).Msg("initializing client set for node metrics, abandoning collection")
		logger.Debug().Str("duration", time.Since(start).String()).Msg("complete")
		return
	}

	req := clientset.CoreV1().RESTClient().Get().RequestURI(nc.baseURI + "/proxy/metrics")
	res := req.Do(nc.ctx)
	data, err := res.Raw()
	if err != nil {
		nc.check.IncrementCounter("collect_api_errors", cgm.Tags{
			cgm.Tag{Category: "source", Value: release.NAME},
			cgm.Tag{Category: "request", Value: "metrics"},
			cgm.Tag{Category: "proxy", Value: "api-server"},
			cgm.Tag{Category: "target", Value: "kubelet"},
		})
		logger.Error().Err(err).Str("url", req.URL().String()).Msg("fetching /metrics stats")
		logger.Debug().Str("duration", time.Since(start).String()).Msg("complete")
		return
	}

	nc.check.AddHistSample("collect_latency", cgm.Tags{
		cgm.Tag{Category: "source", Value: release.NAME},
		cgm.Tag{Category: "request", Value: "metrics"},
		cgm.Tag{Category: "proxy", Value: "api-server"},
		cgm.Tag{Category: "target", Value: "kubelet"},
		cgm.Tag{Category: "units", Value: "milliseconds"},
	}, float64(time.Since(start).Milliseconds()))

	var parser expfmt.TextParser
	if err := promtext.QueueMetrics(nc.ctx, parser, nc.check, logger, bytes.NewReader(data), parentStreamTags, parentMeasurementTags, nil); err != nil {
		logger.Error().Err(err).Msg("parsing node metrics")
	}
	logger.Debug().Str("duration", time.Since(start).String()).Msg("complete")
}

// cadvisor emits metrics from the node /metrics/cadvisor endpoint
func (nc *Collector) cadvisor(parentStreamTags []string, parentMeasurementTags []string) {
	if nc.done() {
		return
	}

	start := time.Now()
	logger := nc.log.With().Str("type", "/metrics/cadvisor").Logger()
	logger.Debug().Msg("start")

	clientset, err := k8s.GetClient(&nc.cfg)
	if err != nil {
		logger.Error().Err(err).Msg("initializing client set for cadvisor metrics, abandoning collection")
		logger.Debug().Str("duration", time.Since(start).String()).Msg("complete")
		return
	}

	req := clientset.CoreV1().RESTClient().Get().RequestURI(nc.baseURI + "/proxy/metrics/cadvisor")
	res := req.Do(nc.ctx)
	data, err := res.Raw()
	if err != nil {
		nc.check.IncrementCounter("collect_api_errors", cgm.Tags{
			cgm.Tag{Category: "source", Value: release.NAME},
			cgm.Tag{Category: "request", Value: "metrics/cadvisor"},
			cgm.Tag{Category: "proxy", Value: "api-server"},
			cgm.Tag{Category: "target", Value: "kubelet"},
		})
		logger.Error().Err(err).Str("url", req.URL().String()).Msg("fetching /metrics/cadvisor stats")
		logger.Debug().Str("duration", time.Since(start).String()).Msg("complete")
		return
	}

	nc.check.AddHistSample("collect_latency", cgm.Tags{
		cgm.Tag{Category: "source", Value: release.NAME},
		cgm.Tag{Category: "request", Value: "metrics/cadvisor"},
		cgm.Tag{Category: "proxy", Value: "api-server"},
		cgm.Tag{Category: "target", Value: "kubelet"},
		cgm.Tag{Category: "units", Value: "milliseconds"},
	}, float64(time.Since(start).Milliseconds()))

	streamTags := nc.check.NewTagList(parentStreamTags, []string{"__rollup:false"})
	var parser expfmt.TextParser
	if err := promtext.QueueMetrics(nc.ctx, parser, nc.check, logger, bytes.NewReader(data), streamTags, parentMeasurementTags, nil); err != nil {
		logger.Error().Err(err).Msg("parsing node metrics/cadvisor")
	}
	logger.Debug().Str("duration", time.Since(start).String()).Msg("complete")
}
