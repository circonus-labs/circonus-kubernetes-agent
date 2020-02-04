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
	"io/ioutil"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/circonus-labs/circonus-kubernetes-agent/internal/circonus"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/config"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/k8s"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/promtext"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	"k8s.io/apimachinery/pkg/api/resource"
)

type Collector struct {
	cfg        *config.Cluster
	tlsConfig  *tls.Config
	ctx        context.Context
	check      *circonus.Check
	node       *k8s.Node
	baseLogger zerolog.Logger
	log        zerolog.Logger
	ts         *time.Time
}

func New(cfg *config.Cluster, node *k8s.Node, logger zerolog.Logger, check *circonus.Check) (*Collector, error) {
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
		cfg:        cfg,
		check:      check,
		node:       node,
		baseLogger: logger.With().Str("node", node.Metadata.Name).Logger(),
	}, nil
}

func (nc *Collector) Collect(ctx context.Context, workerID int, tlsConfig *tls.Config, ts *time.Time) {
	nc.ctx = ctx
	nc.tlsConfig = tlsConfig
	nc.ts = ts
	nc.log = nc.baseLogger.With().Int("worker_id", workerID).Logger()

	collectStart := time.Now()

	baseMeasurementTags := []string{}
	baseStreamTags := []string{"source:kubelet", "node:" + nc.node.Metadata.Name}

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

	wg.Wait()

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

	if nc.check.StreamMetrics() {

		var buf bytes.Buffer

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
			_ = nc.check.WriteMetricSample(
				&buf,
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
				_ = nc.check.WriteMetricSample(
					&buf,
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
				_ = nc.check.WriteMetricSample(
					&buf,
					"capacity_cpu",
					circonus.MetricTypeUint64,
					streamTags, parentMeasurementTags,
					uint64(v),
					nil)
			} else {
				nc.log.Warn().Err(err).Str("cpu", nc.node.Status.Capacity.CPU).Msg("converting capacity.cpu")
			}
			if v, err := strconv.Atoi(nc.node.Status.Capacity.Pods); err == nil {
				_ = nc.check.WriteMetricSample(
					&buf,
					"capacity_pods",
					circonus.MetricTypeUint64,
					streamTags, parentMeasurementTags,
					uint64(v),
					nil)
			} else {
				nc.log.Warn().Err(err).Str("pods", nc.node.Status.Capacity.Pods).Msg("converting capacity.pods")
			}
			if qty, err := resource.ParseQuantity(nc.node.Status.Capacity.Memory); err == nil {
				if mem, ok := qty.AsInt64(); ok {
					streamTags = append(streamTags, "units:bytes")
					_ = nc.check.WriteMetricSample(
						&buf,
						"capacity_memory",
						circonus.MetricTypeUint64,
						streamTags, parentMeasurementTags,
						uint64(mem),
						nil)
				}
			} else {
				nc.log.Warn().Err(err).Str("memory", nc.node.Status.Capacity.Memory).Msg("parsing quantity capacity.memory")
			}
			if qty, err := resource.ParseQuantity(nc.node.Status.Capacity.EphemeralStorage); err == nil {
				if storage, ok := qty.AsInt64(); ok {
					streamTags = append(streamTags, "units:bytes")
					_ = nc.check.WriteMetricSample(
						&buf,
						"capacity_ephemeral_storage",
						circonus.MetricTypeUint64,
						streamTags, parentMeasurementTags,
						uint64(storage),
						nil)
				}
			} else {
				nc.log.Warn().Err(err).Str("ephemeral_storage", nc.node.Status.Capacity.EphemeralStorage).Msg("parsing quantity capacity.ephemeral-storage")
			}
		}

		if buf.Len() == 0 {
			nc.log.Warn().Msg("no telemetry to submit")
			return
		}
		if err := nc.check.SubmitStream(nc.ctx, &buf, nc.log.With().Str("type", "meta").Logger()); err != nil {
			nc.log.Warn().Err(err).Msg("submitting metrics")
		}
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
			nil)
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
				nil)
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
				nil)
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
				nil)
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
					nil)
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
					nil)
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

	client, err := k8s.NewAPIClient(nc.tlsConfig)
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

	resp, err := client.Do(req)
	if err != nil {
		nc.log.Error().Err(err).Str("req_url", reqURL).Msg("fetching summary stats")
		return
	}

	defer resp.Body.Close()
	if nc.done() {
		return
	}

	if resp.StatusCode != http.StatusOK {
		data, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			nc.log.Error().Err(err).Str("url", reqURL).Msg("reading response")
			return
		}
		nc.log.Warn().Str("url", reqURL).Str("status", resp.Status).RawJSON("response", data).Msg("error from API server")
		return
	}

	var stats statsSummary
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
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

	if nc.check.StreamMetrics() {
		var buf bytes.Buffer

		nc.streamCPU(&buf, &node.CPU, parentStreamTags, parentMeasurementTags)
		nc.streamMemory(&buf, &node.Memory, parentStreamTags, parentMeasurementTags)
		nc.streamNetwork(&buf, &node.Network, parentStreamTags, parentMeasurementTags)
		nc.streamFS(&buf, &node.FS, parentStreamTags, parentMeasurementTags)
		nc.streamRuntimeImageFS(&buf, &node.Runtime.ImageFs, parentStreamTags, parentMeasurementTags)
		nc.streamRlimit(&buf, &node.Rlimit, parentStreamTags, parentMeasurementTags)

		if buf.Len() == 0 {
			nc.log.Warn().Msg("no telemetry to submit")
			return
		}
		if err := nc.check.SubmitStream(nc.ctx, &buf, nc.log.With().Str("type", "/stats/summary").Logger()); err != nil {
			nc.log.Warn().Err(err).Msg("submitting metrics")
		}
		return
	}

	metrics := make(map[string]circonus.MetricSample)

	nc.queueCPU(metrics, &node.CPU, parentStreamTags, parentMeasurementTags)
	nc.queueMemory(metrics, &node.Memory, parentStreamTags, parentMeasurementTags)
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

	if nc.check.StreamMetrics() {
		var buf bytes.Buffer

		for _, container := range node.SystemContainers {
			if nc.done() {
				break
			}
			var streamTags []string
			streamTags = append(streamTags, parentStreamTags...)
			streamTags = append(streamTags, []string{"sys_container:" + container.Name}...)

			nc.streamCPU(&buf, &container.CPU, streamTags, parentMeasurementTags)
			nc.streamMemory(&buf, &container.Memory, streamTags, parentMeasurementTags)
			nc.streamRootFS(&buf, &container.RootFS, streamTags, parentMeasurementTags)
			nc.streamLogsFS(&buf, &container.Logs, streamTags, parentMeasurementTags)
		}

		if buf.Len() == 0 {
			nc.log.Warn().Msg("no telemetry to submit")
			return
		}
		if err := nc.check.SubmitStream(nc.ctx, &buf, nc.log.With().Str("type", "system_containers").Logger()); err != nil {
			nc.log.Warn().Err(err).Msg("submitting metrics")
		}
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
		nc.queueMemory(metrics, &container.Memory, streamTags, parentMeasurementTags)
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

	if nc.check.StreamMetrics() {
		var buf bytes.Buffer

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

			nc.streamCPU(&buf, &pod.CPU, podStreamTags, parentMeasurementTags)
			nc.streamMemory(&buf, &pod.Memory, podStreamTags, parentMeasurementTags)
			nc.streamNetwork(&buf, &pod.Network, podStreamTags, parentMeasurementTags)

			for _, volume := range pod.Volumes {
				if nc.done() {
					break
				}
				volume := volume
				nc.streamVolume(&buf, &volume, podStreamTags, parentMeasurementTags)
			}

			if nc.cfg.IncludeContainers {
				for _, container := range pod.Containers {
					if nc.done() {
						break
					}
					var streamTagList []string
					streamTagList = append(streamTagList, podStreamTags...)
					streamTagList = append(streamTagList, "container_name:"+container.Name)

					nc.streamCPU(&buf, &container.CPU, streamTagList, parentMeasurementTags)
					nc.streamMemory(&buf, &container.Memory, streamTagList, parentMeasurementTags)
					if container.RootFS.CapacityBytes > 0 { // rootfs
						nc.streamRootFS(&buf, &container.RootFS, streamTagList, parentMeasurementTags)
					}

					if container.Logs.CapacityBytes > 0 { // logs
						nc.streamLogsFS(&buf, &container.Logs, streamTagList, parentMeasurementTags)
					}
				}
			}
		}

		if buf.Len() == 0 {
			nc.log.Warn().Msg("no telemetry to submit")
			return
		}
		if err := nc.check.SubmitStream(nc.ctx, &buf, nc.log.With().Str("type", "pods").Logger()); err != nil {
			nc.log.Warn().Err(err).Msg("submitting metrics")
		}
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
		nc.queueMemory(metrics, &pod.Memory, podStreamTags, parentMeasurementTags)
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
				nc.queueMemory(metrics, &container.Memory, streamTagList, parentMeasurementTags)
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

	client, err := k8s.NewAPIClient(nc.tlsConfig)
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

	resp, err := client.Do(req)
	if err != nil {
		nc.log.Error().Err(err).Str("url", reqURL).Msg("node metrics")
		return
	}

	defer resp.Body.Close()
	if nc.done() {
		return
	}

	if resp.StatusCode != http.StatusOK {
		data, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			nc.log.Error().Err(err).Str("url", reqURL).Msg("reading response")
			return
		}
		nc.log.Warn().Str("url", reqURL).Str("status", resp.Status).RawJSON("response", data).Msg("error from API server")
		return
	}

	if nc.check.StreamMetrics() {
		if err := promtext.StreamMetrics(nc.ctx, nc.check, nc.log, resp.Body, nc.check, parentStreamTags, parentMeasurementTags, nc.ts); err != nil {
			nc.log.Error().Err(err).Msg("parsing node metrics")
		}
	} else {
		if err := promtext.QueueMetrics(nc.ctx, nc.check, nc.log, resp.Body, nc.check, parentStreamTags, parentMeasurementTags, nil); err != nil {
			nc.log.Error().Err(err).Msg("parsing node metrics")
		}
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

	client, err := k8s.NewAPIClient(nc.tlsConfig)
	if err != nil {
		return collect, tags, err
	}
	defer client.CloseIdleConnections()

	reqURL := nc.cfg.URL + "/api/v1/namespaces/" + ns + "/pods/" + name
	req, err := k8s.NewAPIRequest(nc.cfg.BearerToken, reqURL)
	if err != nil {
		return collect, tags, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return collect, tags, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			nc.log.Error().Err(err).Str("url", reqURL).Msg("reading response")
			return collect, nil, err
		}
		nc.log.Warn().Str("url", reqURL).Str("status", resp.Status).RawJSON("response", data).Msg("error from API server")
		return collect, nil, errors.Errorf("error from api %s (%s)", resp.Status, string(data))
	}

	var ps podSpec
	if err := json.NewDecoder(resp.Body).Decode(&ps); err != nil {
		return collect, tags, err
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
