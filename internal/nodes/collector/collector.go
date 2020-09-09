// Copyright © 2019 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// Package collector collects metrics from nodes
package collector

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"sync"
	"time"

	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/circonus"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/config"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/k8s"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/promtext"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/release"
	"github.com/pkg/errors"
	"github.com/prometheus/common/expfmt"
	"github.com/rs/zerolog"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Collector struct {
	cfg          *config.Cluster
	tlsConfig    *tls.Config
	ctx          context.Context
	check        *circonus.Check
	node         *v1.Node
	baseLogger   zerolog.Logger
	log          zerolog.Logger
	ts           *time.Time
	apiTimelimit time.Duration
}

func New(cfg *config.Cluster, node *v1.Node, logger zerolog.Logger, check *circonus.Check, apiTimeout time.Duration) (*Collector, error) {
	if cfg == nil {
		return nil, errors.New("invalid cluster config (nil)")
	}
	if node == nil {
		return nil, errors.New("invalid node (nil)")
	}
	if check == nil {
		return nil, errors.New("invalid check (nil)")
	}

	return &Collector{
		cfg:          cfg,
		check:        check,
		node:         node,
		apiTimelimit: apiTimeout,
		baseLogger:   logger.With().Str("node", node.Name).Logger(),
	}, nil
}

func (nc *Collector) Collect(ctx context.Context, workerID int, tlsConfig *tls.Config, ts *time.Time, concurrent bool) {
	nc.ctx = ctx
	nc.tlsConfig = tlsConfig
	nc.ts = ts
	nc.log = nc.baseLogger.With().Int("worker_id", workerID).Logger()

	collectStart := time.Now()

	baseMeasurementTags := []string{}
	baseStreamTags := []string{"source:kubelet", "node:" + nc.node.Name}

	if concurrent {
		var wg sync.WaitGroup

		wg.Add(1)
		go func() {
			nc.meta(baseStreamTags, baseMeasurementTags) // from node list
			wg.Done()
		}()
		if nc.cfg.EnableNodeStats {
			wg.Add(1)
			go func() {
				nc.summary(baseStreamTags, baseMeasurementTags) // from /stats/summary
				wg.Done()
			}()
		}
		if nc.cfg.EnableNodeMetrics {
			wg.Add(1)
			go func() {
				nc.nmetrics(baseStreamTags, baseMeasurementTags) // from /metrics
				wg.Done()
			}()
		}
		if nc.cfg.EnableCadvisorMetrics {
			wg.Add(1)
			go func() {
				nc.cadvisor(baseStreamTags, baseMeasurementTags) // from /metrics/cadvisor
				wg.Done()
			}()
		}

		wg.Wait()
	} else {
		nc.meta(baseStreamTags, baseMeasurementTags) // from node list
		if nc.cfg.EnableNodeStats {
			nc.summary(baseStreamTags, baseMeasurementTags) // from /stats/summary
		}
		if nc.cfg.EnableNodeMetrics {
			nc.nmetrics(baseStreamTags, baseMeasurementTags) // from /metrics
		}
		if nc.cfg.EnableCadvisorMetrics {
			nc.cadvisor(baseStreamTags, baseMeasurementTags) // from /metrics/cadvisor
		}
	}

	nc.check.AddHistSample("collect_latency", cgm.Tags{
		cgm.Tag{Category: "op", Value: "collect_node"},
		cgm.Tag{Category: "source", Value: release.NAME},
		cgm.Tag{Category: "units", Value: "milliseconds"},
	}, float64(time.Since(collectStart).Milliseconds()))
	nc.log.
		Debug().
		Str("duration", time.Since(collectStart).String()).
		Msg("node collect end")
}

// meta emits node meta stats
func (nc *Collector) meta(parentStreamTags []string, parentMeasurementTags []string) {
	if nc.done() {
		return
	}

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
			circonus.MetricTypeString,
			streamTags, parentMeasurementTags,
			nc.node.Name,
			nc.ts)
	}

	{ // conditions
		streamTags := nc.check.NewTagList(parentStreamTags, []string{"status:condition"})
		for _, cond := range nc.node.Status.Conditions {
			if nc.done() {
				break
			}
			_ = nc.check.QueueMetricSample(
				metrics,
				string(cond.Type),
				circonus.MetricTypeString,
				streamTags, parentMeasurementTags,
				cond.Message,
				nc.ts)
		}
	}

	{ // capacity and allocatable
		streamTags := nc.check.NewTagList(parentStreamTags)
		cpu := int64(0)
		if qty, err := resource.ParseQuantity(nc.node.Status.Capacity.Cpu().String()); err == nil {
			if cpu, ok := qty.AsInt64(); ok {
				_ = nc.check.QueueMetricSample(
					metrics,
					"capacity_cpu",
					circonus.MetricTypeUint64,
					streamTags, parentMeasurementTags,
					uint64(cpu),
					nc.ts)
				if ns, ok := GetNodeStat(nc.node.Name); ok {
					ns.CPUCapacity = uint64(cpu)
					SetNodeStat(nc.node.Name, ns)
				} else {
					ns := NodeStat{
						CPUCapacity: uint64(cpu),
					}
					SetNodeStat(nc.node.Name, ns)
				}
			}
		} else {
			nc.log.Warn().Err(err).Str("cpu", nc.node.Status.Capacity.Cpu().String()).Msg("converting capacity.cpu")
		}
		if ns, ok := GetNodeStat(nc.node.Name); ok {
			ns.CPUCapacity = uint64(cpu)
			SetNodeStat(nc.node.Name, ns)
		} else {
			ns := NodeStat{
				CPUCapacity: uint64(cpu),
			}
			SetNodeStat(nc.node.Name, ns)
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
			nc.log.Warn().Err(err).Str("pods", nc.node.Status.Capacity.Pods().String()).Msg("converting capacity.pods")
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
			nc.log.Warn().Err(err).Str("memory", nc.node.Status.Capacity.Memory().String()).Msg("parsing quantity capacity.memory")
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
			nc.log.Warn().Err(err).Str("ephemeral_storage", nc.node.Status.Capacity.StorageEphemeral().String()).Msg("parsing quantity capacity.ephemeral-storage")
		}
	}

	if len(metrics) == 0 {
		nc.log.Warn().Msg("no meta telemetry to submit")
		return
	}
	if err := nc.check.SubmitMetrics(nc.ctx, metrics, nc.log.With().Str("type", "meta").Logger(), true); err != nil {
		nc.log.Warn().Err(err).Msg("submitting metrics")
	}

}

type statsSummary struct {
	Node statsSummaryNode `json:"node"`
	Pods []pod            `json:"pods"`
}

type statsSummaryNode struct {
	NodeName         string      `json:"nodeName"`
	SystemContainers []container `json:"systemContainers"`
	CPU              cpu         `json:"cpu"`
	Memory           memory      `json:"memory"`
	Network          network     `json:"network"`
	FS               fs          `json:"fs"`
	Runtime          runtime     `json:"runtime"`
	Rlimit           rlimit      `json:"rlimit"`
}

type cpu struct {
	UsageNanoCores       uint64 `json:"usageNanoCores"`
	UsageCoreNanoSeconds uint64 `json:"usageCoreNanoSeconds"`
}
type memory struct {
	AvailableBytes  uint64 `json:"availableBytes"`
	UsageBytes      uint64 `json:"usageBytes"`
	WorkingSetBytes uint64 `json:"workingSetBytes"`
	RSSBytes        uint64 `json:"rssBytes"`
	PageFaults      uint64 `json:"pageFaults"`
	MajorPageFaults uint64 `json:"majorPageFaults"`
}
type network struct {
	networkInterface
	Interfaces []networkInterface `json:"interfaces"`
}
type networkInterface struct {
	Name     string `json:"name"`
	RxBytes  uint64 `json:"rxBytes"`
	RxErrors uint64 `json:"rxErrors"`
	TxBytes  uint64 `json:"txBytes"`
	TxErrors uint64 `json:"txErrors"`
}
type fs struct {
	AvailableBytes uint64 `json:"availableBytes"`
	CapacityBytes  uint64 `json:"capacityBytes"`
	UsedBytes      uint64 `json:"usedBytes"`
	InodesFree     uint64 `json:"inodesFree"`
	Inodes         uint64 `json:"inodes"`
	InodesUsed     uint64 `json:"inodesUsed"`
}
type runtime struct {
	ImageFs fs `json:"imageFs"`
}
type rlimit struct {
	MaxPID  uint64 `json:"maxpid"`
	CurProc uint64 `json:"curproc"`
}
type pod struct {
	PodRef           podRef      `json:"podRef"`
	Containers       []container `json:"containers"`
	CPU              cpu         `json:"cpu"`
	Memory           memory      `json:"memory"`
	Network          network     `json:"network"`
	Volumes          []volume    `json:"volume"`
	EphemeralStorage fs          `json:"ephemeral-storage"`
}
type podRef struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}
type container struct {
	Name   string `json:"name"`
	CPU    cpu    `json:"cpu"`
	Memory memory `json:"memory"`
	RootFS fs     `json:"rootfs"`
	Logs   fs     `json:"logs"`
}
type volume struct {
	Name string `json:"name"`
	fs
}

// summary emits node summary stats
func (nc *Collector) summary(parentStreamTags []string, parentMeasurementTags []string) {
	if nc.done() {
		return
	}

	start := time.Now()

	clientset, err := k8s.GetClient(nc.cfg)
	if err != nil {
		nc.log.Error().Err(err).Msg("initializing client set for stats/summary, abandoning collection")
		return
	}

	req := clientset.CoreV1().RESTClient().Get().RequestURI(nc.node.SelfLink + "/proxy/stats/summary")
	res := req.Do()
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
	nc.queueRuntimeImageFS(metrics, &node.Runtime.ImageFs, parentStreamTags, parentMeasurementTags)
	nc.queueRlimit(metrics, &node.Rlimit, parentStreamTags, parentMeasurementTags)

	if len(metrics) == 0 {
		nc.log.Warn().Msg("no summary telemetry to submit")
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
		nc.log.Warn().Msg("no system container telemetry to submit")
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

	for _, pod := range stats.Pods {
		if nc.done() {
			break
		}
		collect, podLabels, err := nc.getPodLabels(pod.PodRef.Namespace, pod.PodRef.Name)
		if err != nil {
			nc.log.Warn().Err(err).Str("pod", pod.PodRef.Name).Str("ns", pod.PodRef.Namespace).Msg("fetching pod labels")
		}
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
		nc.log.Warn().Msg("no pod telemetry to submit")
		return
	}
	if err := nc.check.SubmitMetrics(nc.ctx, metrics, nc.log.With().Str("type", "pods").Logger(), true); err != nil {
		nc.log.Warn().Err(err).Msg("submitting metrics")
	}
}

// nmetrics emits metrics from the node /metrics endpoint
func (nc *Collector) nmetrics(parentStreamTags []string, parentMeasurementTags []string) {
	if nc.done() {
		return
	}

	start := time.Now()
	clientset, err := k8s.GetClient(nc.cfg)
	if err != nil {
		nc.log.Error().Err(err).Msg("initializing client set for node metrics, abandoning collection")
		return
	}

	req := clientset.CoreV1().RESTClient().Get().RequestURI(nc.node.SelfLink + "/proxy/metrics")
	res := req.Do()
	data, err := res.Raw()
	if err != nil {
		nc.check.IncrementCounter("collect_api_errors", cgm.Tags{
			cgm.Tag{Category: "source", Value: release.NAME},
			cgm.Tag{Category: "request", Value: "metrics"},
			cgm.Tag{Category: "proxy", Value: "api-server"},
			cgm.Tag{Category: "target", Value: "kubelet"},
		})
		nc.log.Error().Err(err).Str("url", req.URL().String()).Msg("fetching /metrics stats")
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
	if err := promtext.QueueMetrics(nc.ctx, parser, nc.check, nc.log, bytes.NewReader(data), parentStreamTags, parentMeasurementTags, nil); err != nil {
		nc.log.Error().Err(err).Msg("parsing node metrics")
	}
}

// cadvisor emits metrics from the node /metrics/cadvisor endpoint
func (nc *Collector) cadvisor(parentStreamTags []string, parentMeasurementTags []string) {
	if nc.done() {
		return
	}

	start := time.Now()

	clientset, err := k8s.GetClient(nc.cfg)
	if err != nil {
		nc.log.Error().Err(err).Msg("initializing client set for cadvisor metrics, abandoning collection")
		return
	}

	req := clientset.CoreV1().RESTClient().Get().RequestURI(nc.node.SelfLink + "/proxy/metrics/cadvisor")
	res := req.Do()
	data, err := res.Raw()
	if err != nil {
		nc.check.IncrementCounter("collect_api_errors", cgm.Tags{
			cgm.Tag{Category: "source", Value: release.NAME},
			cgm.Tag{Category: "request", Value: "metrics/cadvisor"},
			cgm.Tag{Category: "proxy", Value: "api-server"},
			cgm.Tag{Category: "target", Value: "kubelet"},
		})
		nc.log.Error().Err(err).Str("url", req.URL().String()).Msg("fetching /metrics/cadvisor stats")
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
	if err := promtext.QueueMetrics(nc.ctx, parser, nc.check, nc.log, bytes.NewReader(data), streamTags, parentMeasurementTags, nil); err != nil {
		nc.log.Error().Err(err).Msg("parsing node metrics/cadvisor")
	}
}

func (nc *Collector) getPodLabels(ns string, name string) (bool, []string, error) {
	collect := false
	tags := []string{}

	start := time.Now()

	clientset, err := k8s.GetClient(nc.cfg)
	if err != nil {
		nc.log.Error().Err(err).Msg("initializing client set for pod labels, abandoning collection")
		return collect, tags, err
	}

	pod, err := clientset.CoreV1().Pods(ns).Get(name, metav1.GetOptions{})
	if err != nil {
		nc.check.IncrementCounter("collect_api_errors", cgm.Tags{
			cgm.Tag{Category: "source", Value: release.NAME},
			cgm.Tag{Category: "request", Value: "pod"},
			cgm.Tag{Category: "target", Value: "api-server"},
		})
		nc.log.Error().Err(err).Str("pod", name).Str("ns", ns).Msg("fetching pod labels")
		return collect, tags, err
	}

	nc.check.AddHistSample("collect_latency", cgm.Tags{
		cgm.Tag{Category: "request", Value: "pod-labels"},
		cgm.Tag{Category: "target", Value: "api-server"},
		cgm.Tag{Category: "source", Value: release.NAME},
		cgm.Tag{Category: "units", Value: "milliseconds"},
	}, float64(time.Since(start).Milliseconds()))

	collect = true
	if nc.cfg.PodLabelKey != "" {
		collect = false
		if v, ok := pod.Labels[nc.cfg.PodLabelKey]; ok {
			if nc.cfg.PodLabelVal == "" {
				collect = true
			} else if v == nc.cfg.PodLabelVal {
				collect = true
			}
		}
	}

	tags = labelsToTags(pod.Labels)
	return collect, tags, nil
}

func labelsToTags(labels map[string]string) []string {
	tags := make([]string, len(labels))
	idx := 0
	for k, v := range labels {
		tags[idx] = k + ":" + v
		idx++
	}
	return tags
}

func (nc *Collector) done() bool {
	select {
	case <-nc.ctx.Done():
		return true
	default:
		return false
	}
}
