// Copyright © 2019 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package collector

import (
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/circonus"
)

func (nc *Collector) queueCPU(dest map[string]circonus.MetricSample, stats *cpu, isNode bool, parentStreamTags []string, parentMeasurementTags []string) {
	cores := "usageNanoCores"
	seconds := "usageCoreNanoSeconds"
	utilization := "utilization"

	var streamTags []string
	streamTags = append(streamTags, parentStreamTags...)
	streamTags = append(streamTags, "resource:cpu")
	_ = nc.check.QueueMetricSample(dest, cores, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.UsageNanoCores, nc.ts)
	{
		var st []string
		st = append(st, streamTags...)
		st = append(st, "units:seconds")
		_ = nc.check.QueueMetricSample(dest, seconds, circonus.MetricTypeUint64, st, parentMeasurementTags, stats.UsageCoreNanoSeconds, nc.ts)
	}

	// if node, add % utilized
	if isNode {
		if ns, ok := GetNodeStat(nc.node.Name); ok {
			if ns.LastCPUNanoSeconds > 0 {
				var st []string
				st = append(st, streamTags...)
				st = append(st, "units:percent")
				pct := float64(0)

				if ns.LastCPUNanoSeconds > 0 && stats.UsageCoreNanoSeconds > 0 && ns.CPUCapacity > 0 {
					// calc ref: https://github.com/kubernetes-retired/heapster/issues/650#issuecomment-147795824
					usage := float64(stats.UsageCoreNanoSeconds - ns.LastCPUNanoSeconds)
					capacity := float64(ns.CPUCapacity * 1e+9)
					pct = usage / capacity
				}

				_ = nc.check.QueueMetricSample(dest, utilization, circonus.MetricTypeFloat64, st, parentMeasurementTags, pct, nc.ts)
			}

			ns.LastCPUNanoSeconds = stats.UsageCoreNanoSeconds
			SetNodeStat(nc.node.Name, ns)
		}
	}
}

func (nc *Collector) queueMemory(dest map[string]circonus.MetricSample, stats *memory, parentStreamTags []string, parentMeasurementTags []string, isNode bool) {
	available := "available"
	used := "used"
	workingSet := "workingSet"
	rss := "rss"
	pageFaults := "pageFaults"
	majorPageFaults := "majorPageFaults"

	{ // units:bytes
		var streamTags []string
		streamTags = append(streamTags, parentStreamTags...)
		streamTags = append(streamTags, []string{"resource:memory", "units:bytes"}...)
		if isNode {
			// pods don't have 'available'
			_ = nc.check.QueueMetricSample(dest, available, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.AvailableBytes, nc.ts)
		}
		_ = nc.check.QueueMetricSample(dest, used, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.UsageBytes, nc.ts)
		_ = nc.check.QueueMetricSample(dest, workingSet, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.WorkingSetBytes, nc.ts)
		_ = nc.check.QueueMetricSample(dest, rss, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.RSSBytes, nc.ts)
	}
	{ // units:faults
		var streamTags []string
		streamTags = append(streamTags, parentStreamTags...)
		streamTags = append(streamTags, []string{"resource:memory", "units:faults"}...)
		_ = nc.check.QueueMetricSample(dest, pageFaults, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.PageFaults, nc.ts)
		_ = nc.check.QueueMetricSample(dest, majorPageFaults, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.MajorPageFaults, nc.ts)
	}
}

func (nc *Collector) queueNetwork(dest map[string]circonus.MetricSample, stats *network, parentStreamTags []string, parentMeasurementTags []string) {
	receive := "rx"
	transmit := "tx"

	{ // units:bytes
		var streamTags []string
		streamTags = append(streamTags, parentStreamTags...)
		streamTags = append(streamTags, []string{"resource:network", "units:bytes"}...)
		_ = nc.check.QueueMetricSample(dest, receive, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.RxBytes, nc.ts)
		_ = nc.check.QueueMetricSample(dest, transmit, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.TxBytes, nc.ts)
	}
	{ // units:errors
		var streamTags []string
		streamTags = append(streamTags, parentStreamTags...)
		streamTags = append(streamTags, []string{"resource:network", "units:errors"}...)
		_ = nc.check.QueueMetricSample(dest, receive, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.RxErrors, nc.ts)
		_ = nc.check.QueueMetricSample(dest, transmit, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.TxErrors, nc.ts)
	}

	for _, iface := range stats.Interfaces {
		{ // units:bytes
			var streamTags []string
			streamTags = append(streamTags, parentStreamTags...)
			streamTags = append(streamTags, []string{"resource:network", "units:bytes", "interface:" + iface.Name}...)
			_ = nc.check.QueueMetricSample(dest, receive, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, iface.RxBytes, nc.ts)
			_ = nc.check.QueueMetricSample(dest, transmit, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, iface.TxBytes, nc.ts)
		}
		{ // units:errors
			var streamTags []string
			streamTags = append(streamTags, parentStreamTags...)
			streamTags = append(streamTags, []string{"resource:network", "units:errors", "interface:" + iface.Name}...)
			_ = nc.check.QueueMetricSample(dest, receive, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, iface.RxErrors, nc.ts)
			_ = nc.check.QueueMetricSample(dest, transmit, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, iface.TxErrors, nc.ts)
		}
	}
}

func (nc *Collector) queueBaseFS(dest map[string]circonus.MetricSample, stats *fs, parentStreamTags []string, parentMeasurementTags []string) {
	capacity := "capacity"
	free := "free"
	used := "used"

	{ // units:bytes
		var streamTags []string
		streamTags = append(streamTags, parentStreamTags...)
		streamTags = append(streamTags, "units:bytes")
		_ = nc.check.QueueMetricSample(dest, capacity, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.CapacityBytes, nc.ts)
		_ = nc.check.QueueMetricSample(dest, free, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.AvailableBytes, nc.ts)
		_ = nc.check.QueueMetricSample(dest, used, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.UsedBytes, nc.ts)
	}
	{ // units:inodes
		var streamTags []string
		streamTags = append(streamTags, parentStreamTags...)
		streamTags = append(streamTags, "units:inodes")
		_ = nc.check.QueueMetricSample(dest, capacity, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.Inodes, nc.ts)
		_ = nc.check.QueueMetricSample(dest, free, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.InodesFree, nc.ts)
		_ = nc.check.QueueMetricSample(dest, used, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.InodesUsed, nc.ts)
	}
	{ // units:percent
		if stats.UsedBytes > 0 && stats.CapacityBytes > 0 {
			var streamTags []string
			streamTags = append(streamTags, parentStreamTags...)
			streamTags = append(streamTags, "units:percent")
			usedPct := ((float64(stats.UsedBytes) / float64(stats.CapacityBytes)) * 100)
			_ = nc.check.QueueMetricSample(dest, used, circonus.MetricTypeFloat64, streamTags, parentMeasurementTags, usedPct, nc.ts)
		}
	}
}

func (nc *Collector) queueFS(dest map[string]circonus.MetricSample, stats *fs, parentStreamTags []string, parentMeasurementTags []string) {
	var baseStreamTags []string
	if len(parentStreamTags) > 0 {
		baseStreamTags = make([]string, len(parentStreamTags))
		copy(baseStreamTags, parentStreamTags)
	}
	baseStreamTags = append(baseStreamTags, "resource:fs")
	nc.queueBaseFS(dest, stats, baseStreamTags, parentMeasurementTags)
}

func (nc *Collector) queueRuntimeImageFS(dest map[string]circonus.MetricSample, stats *fs, parentStreamTags []string, parentMeasurementTags []string) {
	var baseStreamTags []string
	if len(parentStreamTags) > 0 {
		baseStreamTags = make([]string, len(parentStreamTags))
		copy(baseStreamTags, parentStreamTags)
	}
	baseStreamTags = append(baseStreamTags, "resource:runtime_image_fs")
	nc.queueBaseFS(dest, stats, baseStreamTags, parentMeasurementTags)
}

func (nc *Collector) queueRootFS(dest map[string]circonus.MetricSample, stats *fs, parentStreamTags []string, parentMeasurementTags []string) {
	var baseStreamTags []string
	if len(parentStreamTags) > 0 {
		baseStreamTags = make([]string, len(parentStreamTags))
		copy(baseStreamTags, parentStreamTags)
	}
	baseStreamTags = append(baseStreamTags, "resource:root_fs")
	nc.queueBaseFS(dest, stats, baseStreamTags, parentMeasurementTags)
}

func (nc *Collector) queueLogsFS(dest map[string]circonus.MetricSample, stats *fs, parentStreamTags []string, parentMeasurementTags []string) {
	var baseStreamTags []string
	if len(parentStreamTags) > 0 {
		baseStreamTags = make([]string, len(parentStreamTags))
		copy(baseStreamTags, parentStreamTags)
	}
	baseStreamTags = append(baseStreamTags, "resource:logs_fs")
	nc.queueBaseFS(dest, stats, baseStreamTags, parentMeasurementTags)
}

func (nc *Collector) queueVolume(dest map[string]circonus.MetricSample, stats *volume, parentStreamTags []string, parentMeasurementTags []string) {
	var baseStreamTags []string
	if len(parentStreamTags) > 0 {
		baseStreamTags = make([]string, len(parentStreamTags))
		copy(baseStreamTags, parentStreamTags)
	}
	baseStreamTags = append(baseStreamTags, "volume_name:"+stats.Name)
	nc.queueBaseFS(dest, &stats.fs, baseStreamTags, parentMeasurementTags)
}

func (nc *Collector) queueRlimit(dest map[string]circonus.MetricSample, stats *rlimit, parentStreamTags []string, parentMeasurementTags []string) {
	maxPID := "maxPID"
	curProc := "curProc"

	// units:procs
	var streamTags []string
	streamTags = append(streamTags, parentStreamTags...)
	streamTags = append(streamTags, []string{"resource:rlimit", "units:procs"}...)
	_ = nc.check.QueueMetricSample(dest, maxPID, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.MaxPID, nc.ts)
	_ = nc.check.QueueMetricSample(dest, curProc, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.CurProc, nc.ts)
}
