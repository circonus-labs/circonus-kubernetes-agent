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
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
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

	"k8s.io/apimachinery/pkg/api/resource"
)

type Collector struct {
	cfg          *config.Cluster
	tlsConfig    *tls.Config
	ctx          context.Context
	check        *circonus.Check
	node         *k8s.Node
	baseLogger   zerolog.Logger
	log          zerolog.Logger
	ts           *time.Time
	apiTimelimit time.Duration
}

func New(cfg *config.Cluster, node *k8s.Node, logger zerolog.Logger, check *circonus.Check, apiTimeout time.Duration) (*Collector, error) {
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
		baseLogger:   logger.With().Str("node", node.Metadata.Name).Logger(),
	}, nil
}

func (nc *Collector) Collect(ctx context.Context, workerID int, tlsConfig *tls.Config, ts *time.Time, concurrent bool) {
	nc.ctx = ctx
	nc.tlsConfig = tlsConfig
	nc.ts = ts
	nc.log = nc.baseLogger.With().Int("worker_id", workerID).Logger()

	collectStart := time.Now()

	baseMeasurementTags := []string{}
	baseStreamTags := []string{"source:kubelet", "node:" + nc.node.Metadata.Name}

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
		var streamTags []string
		streamTags = append(streamTags, parentStreamTags...)
		streamTags = append(streamTags, []string{
			"kernel_version:" + nc.node.Status.NodeInfo.KernelVersion,
			"os_image:" + nc.node.Status.NodeInfo.OSImage,
			"kublet_version:" + nc.node.Status.NodeInfo.KubeletVersion,
		}...)
		for k, v := range nc.node.Metadata.Labels {
			streamTags = append(streamTags, k+":"+v)
		}
		_ = nc.check.QueueMetricSample(
			metrics,
			"node",
			circonus.MetricTypeString,
			streamTags, parentMeasurementTags,
			nc.node.Metadata.Name,
			nc.ts)
	}

	{ // conditions
		var streamTags []string
		streamTags = append(streamTags, parentStreamTags...)
		streamTags = append(streamTags, "status:condition")
		for _, cond := range nc.node.Status.Conditions {
			if nc.done() {
				break
			}
			_ = nc.check.QueueMetricSample(
				metrics,
				cond.Type,
				circonus.MetricTypeString,
				streamTags, parentMeasurementTags,
				cond.Message,
				nc.ts)
		}
	}

	{ // capacity and allocatable
		var streamTags []string
		streamTags = append(streamTags, parentStreamTags...)
		if v, err := strconv.Atoi(nc.node.Status.Capacity.CPU); err == nil {
			_ = nc.check.QueueMetricSample(
				metrics,
				"capacity_cpu",
				circonus.MetricTypeUint64,
				streamTags, parentMeasurementTags,
				uint64(v),
				nc.ts)
		} else {
			nc.log.Warn().Err(err).Str("cpu", nc.node.Status.Capacity.CPU).Msg("converting capacity.cpu")
		}
		if v, err := strconv.Atoi(nc.node.Status.Capacity.Pods); err == nil {
			_ = nc.check.QueueMetricSample(
				metrics,
				"capacity_pods",
				circonus.MetricTypeUint64,
				streamTags, parentMeasurementTags,
				uint64(v),
				nc.ts)
		} else {
			nc.log.Warn().Err(err).Str("pods", nc.node.Status.Capacity.Pods).Msg("converting capacity.pods")
		}
		if qty, err := resource.ParseQuantity(nc.node.Status.Capacity.Memory); err == nil {
			if mem, ok := qty.AsInt64(); ok {
				streamTags = append(streamTags, "units:bytes")
				_ = nc.check.QueueMetricSample(
					metrics,
					"capacity_memory",
					circonus.MetricTypeUint64,
					streamTags, parentMeasurementTags,
					uint64(mem),
					nc.ts)
			}
		} else {
			nc.log.Warn().Err(err).Str("memory", nc.node.Status.Capacity.Memory).Msg("parsing quantity capacity.memory")
		}
		if qty, err := resource.ParseQuantity(nc.node.Status.Capacity.EphemeralStorage); err == nil {
			if storage, ok := qty.AsInt64(); ok {
				streamTags = append(streamTags, "units:bytes")
				_ = nc.check.QueueMetricSample(
					metrics,
					"capacity_ephemeral_storage",
					circonus.MetricTypeUint64,
					streamTags, parentMeasurementTags,
					uint64(storage),
					nc.ts)
			}
		} else {
			nc.log.Warn().Err(err).Str("ephemeral_storage", nc.node.Status.Capacity.EphemeralStorage).Msg("parsing quantity capacity.ephemeral-storage")
		}
	}

	if len(metrics) == 0 {
		nc.log.Warn().Msg("no telemetry to submit")
		return
	}
	if err := nc.check.SubmitQueue(nc.ctx, metrics, nc.log.With().Str("type", "meta").Logger()); err != nil {
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

	client, err := k8s.NewAPIClient(nc.tlsConfig, nc.apiTimelimit)
	if err != nil {
		nc.log.Error().Err(err).Msg("abandoning collection")
		return
	}
	defer client.CloseIdleConnections()

	reqURL := nc.cfg.URL + nc.node.Metadata.SelfLink + "/proxy/stats/summary"
	req, err := k8s.NewAPIRequest(nc.cfg.BearerToken, reqURL)
	if err != nil {
		nc.log.Error().Err(err).Msg("abandoning collection")
		return
	}

	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		nc.check.IncrementCounter("collect_api_errors", cgm.Tags{
			cgm.Tag{Category: "source", Value: release.NAME},
			cgm.Tag{Category: "request", Value: "stats/summary"},
			cgm.Tag{Category: "proxy", Value: "api-server"},
			cgm.Tag{Category: "target", Value: "kubelet"},
		})
		nc.log.Error().Err(err).Str("req_url", reqURL).Msg("fetching summary stats")
		return
	}
	nc.check.AddHistSample("collect_latency", cgm.Tags{
		cgm.Tag{Category: "source", Value: release.NAME},
		cgm.Tag{Category: "request", Value: "stats/summary"},
		cgm.Tag{Category: "proxy", Value: "api-server"},
		cgm.Tag{Category: "target", Value: "kubelet"},
		cgm.Tag{Category: "units", Value: "milliseconds"},
	}, float64(time.Since(start).Milliseconds()))

	defer resp.Body.Close()
	if nc.done() {
		return
	}

	if resp.StatusCode != http.StatusOK {
		nc.check.IncrementCounter("collect_api_errors", cgm.Tags{
			cgm.Tag{Category: "source", Value: release.NAME},
			cgm.Tag{Category: "request", Value: "stats/summary"},
			cgm.Tag{Category: "proxy", Value: "api-server"},
			cgm.Tag{Category: "target", Value: "kubelet"},
			cgm.Tag{Category: "code", Value: fmt.Sprintf("%d", resp.StatusCode)},
		})
		data, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			nc.log.Error().Err(err).Str("url", reqURL).Msg("reading response")
			return
		}
		nc.log.Warn().Str("url", reqURL).Str("status", resp.Status).RawJSON("response", data).Msg("error from API server")
		return
	}

	var stats statsSummary
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		nc.log.Error().Err(err).Msg("reading summary stats")
		return
	}
	if err := json.Unmarshal(data, &stats); err != nil {
		nc.log.Error().Err(err).Msg("parsing summary stats")
		return
	}

	nc.summaryNode(&stats.Node, parentStreamTags, parentMeasurementTags)
	nc.summarySystemContainers(&stats.Node, parentStreamTags, parentMeasurementTags)
	nc.summaryPods(&stats, parentStreamTags, parentMeasurementTags)
}

func (nc *Collector) summaryNode(node *statsSummaryNode, parentStreamTags []string, parentMeasurementTags []string) {
	if nc.done() {
		return
	}

	metrics := make(map[string]circonus.MetricSample)

	nc.queueCPU(metrics, &node.CPU, parentStreamTags, parentMeasurementTags)
	nc.queueMemory(metrics, &node.Memory, parentStreamTags, parentMeasurementTags, true)
	nc.queueNetwork(metrics, &node.Network, parentStreamTags, parentMeasurementTags)
	nc.queueFS(metrics, &node.FS, parentStreamTags, parentMeasurementTags)
	nc.queueRuntimeImageFS(metrics, &node.Runtime.ImageFs, parentStreamTags, parentMeasurementTags)
	nc.queueRlimit(metrics, &node.Rlimit, parentStreamTags, parentMeasurementTags)

	if len(metrics) == 0 {
		nc.log.Warn().Msg("no telemetry to submit")
		return
	}
	if err := nc.check.SubmitQueue(nc.ctx, metrics, nc.log.With().Str("type", "/stats/summary").Logger()); err != nil {
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
		var streamTags []string
		streamTags = append(streamTags, parentStreamTags...)
		streamTags = append(streamTags, []string{"sys_container:" + container.Name}...)
		nc.queueCPU(metrics, &container.CPU, streamTags, parentMeasurementTags)
		nc.queueMemory(metrics, &container.Memory, streamTags, parentMeasurementTags, false)
		nc.queueRootFS(metrics, &container.RootFS, streamTags, parentMeasurementTags)
		nc.queueLogsFS(metrics, &container.Logs, streamTags, parentMeasurementTags)
	}

	if len(metrics) == 0 {
		nc.log.Warn().Msg("no telemetry to submit")
		return
	}
	if err := nc.check.SubmitQueue(nc.ctx, metrics, nc.log.With().Str("type", "system_containers").Logger()); err != nil {
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
		var podStreamTags []string
		podStreamTags = append(podStreamTags, parentStreamTags...)
		podStreamTags = append(podStreamTags, podLabels...)
		podStreamTags = append(podStreamTags, []string{
			"pod:" + pod.PodRef.Name,
			"namespace:" + pod.PodRef.Namespace,
			"__rollup:false", // prevent high cardinality metrics from rolling up
		}...)

		nc.queueCPU(metrics, &pod.CPU, podStreamTags, parentMeasurementTags)
		nc.queueMemory(metrics, &pod.Memory, podStreamTags, parentMeasurementTags, false)
		nc.queueNetwork(metrics, &pod.Network, podStreamTags, parentMeasurementTags)

		for _, volume := range pod.Volumes {
			if nc.done() {
				break
			}
			volume := volume
			nc.queueVolume(metrics, &volume, podStreamTags, parentMeasurementTags)
		}

		if nc.cfg.IncludeContainers {
			for _, container := range pod.Containers {
				if nc.done() {
					break
				}
				var streamTagList []string
				streamTagList = append(streamTagList, podStreamTags...)
				streamTagList = append(streamTagList, "container_name:"+container.Name)

				nc.queueCPU(metrics, &container.CPU, streamTagList, parentMeasurementTags)
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
		nc.log.Warn().Msg("no telemetry to submit")
		return
	}
	if err := nc.check.SubmitQueue(nc.ctx, metrics, nc.log.With().Str("type", "pods").Logger()); err != nil {
		nc.log.Warn().Err(err).Msg("submitting metrics")
	}
}

// nmetrics emits metrics from the node /metrics endpoint
func (nc *Collector) nmetrics(parentStreamTags []string, parentMeasurementTags []string) {
	if nc.done() {
		return
	}

	client, err := k8s.NewAPIClient(nc.tlsConfig, nc.apiTimelimit)
	if err != nil {
		nc.log.Error().Err(err).Msg("abandoning /metrics collection")
		return
	}
	defer client.CloseIdleConnections()

	reqURL := nc.cfg.URL + nc.node.Metadata.SelfLink + "/proxy/metrics"
	req, err := k8s.NewAPIRequest(nc.cfg.BearerToken, reqURL)
	if err != nil {
		nc.log.Error().Err(err).Msg("abandoning /metrics collection")
		return
	}

	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		nc.check.IncrementCounter("collect_api_errors", cgm.Tags{
			cgm.Tag{Category: "source", Value: release.NAME},
			cgm.Tag{Category: "request", Value: "metrics"},
			cgm.Tag{Category: "proxy", Value: "api-server"},
			cgm.Tag{Category: "target", Value: "kubelet"},
		})
		nc.log.Error().Err(err).Str("url", reqURL).Msg("node metrics")
		return
	}
	nc.check.AddHistSample("collect_latency", cgm.Tags{
		cgm.Tag{Category: "request", Value: "metrics"},
		cgm.Tag{Category: "proxy", Value: "api-server"},
		cgm.Tag{Category: "target", Value: "kubelet"},
		cgm.Tag{Category: "units", Value: "milliseconds"},
	}, float64(time.Since(start).Milliseconds()))

	defer resp.Body.Close()
	if nc.done() {
		return
	}

	if resp.StatusCode != http.StatusOK {
		nc.check.IncrementCounter("collect_api_errors", cgm.Tags{
			cgm.Tag{Category: "source", Value: release.NAME},
			cgm.Tag{Category: "request", Value: "metrics"},
			cgm.Tag{Category: "proxy", Value: "api-server"},
			cgm.Tag{Category: "target", Value: "kubelet"},
			cgm.Tag{Category: "code", Value: fmt.Sprintf("%d", resp.StatusCode)},
		})
		data, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			nc.log.Error().Err(err).Str("url", reqURL).Msg("reading response")
			return
		}
		nc.log.Warn().Str("url", reqURL).Str("status", resp.Status).RawJSON("response", data).Msg("error from API server")
		return
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		nc.log.Error().Err(err).Msg("reading metrics")
		return
	}

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

	client, err := k8s.NewAPIClient(nc.tlsConfig, nc.apiTimelimit)
	if err != nil {
		nc.log.Error().Err(err).Msg("abandoning /metrics/cadvisor collection")
		return
	}
	defer client.CloseIdleConnections()

	reqURL := nc.cfg.URL + nc.node.Metadata.SelfLink + "/proxy/metrics/cadvisor"
	req, err := k8s.NewAPIRequest(nc.cfg.BearerToken, reqURL)
	if err != nil {
		nc.log.Error().Err(err).Msg("abandoning /metrics/cadvisor collection")
		return
	}

	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		nc.check.IncrementCounter("collect_api_errors", cgm.Tags{
			cgm.Tag{Category: "source", Value: release.NAME},
			cgm.Tag{Category: "request", Value: "metrics/cadvisor"},
			cgm.Tag{Category: "proxy", Value: "api-server"},
			cgm.Tag{Category: "target", Value: "kubelet"},
		})
		nc.log.Error().Err(err).Str("url", reqURL).Msg("node metrics/cadvisor")
		return
	}
	nc.check.AddHistSample("collect_latency", cgm.Tags{
		cgm.Tag{Category: "request", Value: "metrics/cadvsior"},
		cgm.Tag{Category: "proxy", Value: "api-server"},
		cgm.Tag{Category: "target", Value: "kubelet"},
		cgm.Tag{Category: "units", Value: "milliseconds"},
	}, float64(time.Since(start).Milliseconds()))

	defer resp.Body.Close()
	if nc.done() {
		return
	}

	if resp.StatusCode != http.StatusOK {
		nc.check.IncrementCounter("collect_api_errors", cgm.Tags{
			cgm.Tag{Category: "source", Value: release.NAME},
			cgm.Tag{Category: "request", Value: "metrics/cadvisor"},
			cgm.Tag{Category: "proxy", Value: "api-server"},
			cgm.Tag{Category: "target", Value: "kubelet"},
			cgm.Tag{Category: "code", Value: fmt.Sprintf("%d", resp.StatusCode)},
		})
		data, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			nc.log.Error().Err(err).Str("url", reqURL).Msg("reading response")
			return
		}
		nc.log.Warn().Str("url", reqURL).Str("status", resp.Status).RawJSON("response", data).Msg("error from API server")
		return
	}

	streamTags := []string{"__rollup:false"} // prevent high cardinality metrics from rolling up
	streamTags = append(streamTags, parentStreamTags...)

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		nc.log.Error().Err(err).Msg("reading metrics")
		return
	}

	var parser expfmt.TextParser
	if err := promtext.QueueMetrics(nc.ctx, parser, nc.check, nc.log, bytes.NewReader(data), streamTags, parentMeasurementTags, nil); err != nil {
		nc.log.Error().Err(err).Msg("parsing node metrics/cadvisor")
	}
}

type podSpec struct {
	Metadata podMeta `json:"metadata"`
}
type podMeta struct {
	Labels map[string]string `json:"labels"`
}

func (nc *Collector) getPodLabels(ns string, name string) (bool, []string, error) {
	collect := false
	tags := []string{}

	client, err := k8s.NewAPIClient(nc.tlsConfig, nc.apiTimelimit)
	if err != nil {
		return collect, tags, err
	}
	defer client.CloseIdleConnections()

	reqURL := nc.cfg.URL + "/api/v1/namespaces/" + ns + "/pods/" + name
	req, err := k8s.NewAPIRequest(nc.cfg.BearerToken, reqURL)
	if err != nil {
		return collect, tags, err
	}

	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		nc.check.IncrementCounter("collect_api_errors", cgm.Tags{
			cgm.Tag{Category: "source", Value: release.NAME},
			cgm.Tag{Category: "request", Value: "pod-labels"},
			cgm.Tag{Category: "target", Value: "api-server"},
		})
		return collect, tags, err
	}
	defer resp.Body.Close()
	nc.check.AddHistSample("collect_latency", cgm.Tags{
		cgm.Tag{Category: "request", Value: "pod-labels"},
		cgm.Tag{Category: "target", Value: "api-server"},
		cgm.Tag{Category: "source", Value: release.NAME},
		cgm.Tag{Category: "units", Value: "milliseconds"},
	}, float64(time.Since(start).Milliseconds()))

	if resp.StatusCode != http.StatusOK {
		nc.check.IncrementCounter("collect_api_errors", cgm.Tags{
			cgm.Tag{Category: "source", Value: release.NAME},
			cgm.Tag{Category: "request", Value: "pod-labels"},
			cgm.Tag{Category: "target", Value: "api-server"},
			cgm.Tag{Category: "code", Value: fmt.Sprintf("%d", resp.StatusCode)},
		})
		data, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			nc.log.Error().Err(err).Str("url", reqURL).Msg("reading response")
			return collect, nil, err
		}
		nc.log.Warn().Str("url", reqURL).Str("status", resp.Status).RawJSON("response", data).Msg("error from API server")
		return collect, nil, errors.Errorf("error from api %s (%s)", resp.Status, string(data))
	}

	var ps podSpec
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return collect, tags, errors.Wrap(err, "reading pod spec body")
	}
	if err := json.Unmarshal(data, &ps); err != nil {
		return collect, tags, errors.Wrap(err, "parsing pod spec json")
	}

	collect = true
	if nc.cfg.PodLabelKey != "" {
		collect = false
		if v, ok := ps.Metadata.Labels[nc.cfg.PodLabelKey]; ok {
			if nc.cfg.PodLabelVal == "" {
				collect = true
			} else if v == nc.cfg.PodLabelVal {
				collect = true
			}
		}
	}

	for k, v := range ps.Metadata.Labels {
		tags = append(tags, k+":"+v)
	}

	return collect, tags, nil
}

func (nc *Collector) done() bool {
	select {
	case <-nc.ctx.Done():
		return true
	default:
		return false
	}
}
