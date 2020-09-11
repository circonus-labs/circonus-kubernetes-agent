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

	streamTags := nc.check.NewTagList(parentStreamTags, []string{"resource:cpu"})
	_ = nc.check.QueueMetricSample(dest, cores, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.UsageNanoCores, nc.ts)
	{
		st := nc.check.NewTagList(streamTags, []string{"units:seconds"})
		_ = nc.check.QueueMetricSample(dest, seconds, circonus.MetricTypeUint64, st, parentMeasurementTags, stats.UsageCoreNanoSeconds, nc.ts)
	}

	// if node, add % utilized
	if isNode {
		if ns, ok := GetNodeStat(nc.node.Name); ok {
			if ns.LastCPUNanoSeconds > 0 {
				st := nc.check.NewTagList(streamTags, []string{"units:percent"})
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
		streamTags := nc.check.NewTagList(parentStreamTags, []string{"resource:memory", "units:bytes"})
		if isNode {
			// pods don't have 'available'
			_ = nc.check.QueueMetricSample(dest, available, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.AvailableBytes, nc.ts)
		}
		_ = nc.check.QueueMetricSample(dest, used, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.UsageBytes, nc.ts)
		_ = nc.check.QueueMetricSample(dest, workingSet, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.WorkingSetBytes, nc.ts)
		_ = nc.check.QueueMetricSample(dest, rss, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.RSSBytes, nc.ts)
	}
	{ // units:faults
		streamTags := nc.check.NewTagList(parentStreamTags, []string{"resource:memory", "units:faults"})
		_ = nc.check.QueueMetricSample(dest, pageFaults, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.PageFaults, nc.ts)
		_ = nc.check.QueueMetricSample(dest, majorPageFaults, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.MajorPageFaults, nc.ts)
	}
}

func (nc *Collector) queueNetwork(dest map[string]circonus.MetricSample, stats *network, parentStreamTags []string, parentMeasurementTags []string) {
	receive := "rx"
	transmit := "tx"

	{ // units:bytes
		streamTags := nc.check.NewTagList(parentStreamTags, []string{"resource:network", "units:bytes"})
		_ = nc.check.QueueMetricSample(dest, receive, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.RxBytes, nc.ts)
		_ = nc.check.QueueMetricSample(dest, transmit, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.TxBytes, nc.ts)
	}
	{ // units:errors
		streamTags := nc.check.NewTagList(parentStreamTags, []string{"resource:network", "units:errors"})
		_ = nc.check.QueueMetricSample(dest, receive, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.RxErrors, nc.ts)
		_ = nc.check.QueueMetricSample(dest, transmit, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.TxErrors, nc.ts)
	}

	for _, iface := range stats.Interfaces {
		ifaceTags := nc.check.NewTagList(parentStreamTags, []string{"resource:network", "interface:" + iface.Name})

		{ // units:bytes
			streamTags := nc.check.NewTagList(ifaceTags, []string{"units:bytes"})
			_ = nc.check.QueueMetricSample(dest, receive, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, iface.RxBytes, nc.ts)
			_ = nc.check.QueueMetricSample(dest, transmit, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, iface.TxBytes, nc.ts)
		}
		{ // units:errors
			streamTags := nc.check.NewTagList(ifaceTags, []string{"units:errors"})
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
		streamTags := nc.check.NewTagList(parentStreamTags, []string{"units:bytes"})
		_ = nc.check.QueueMetricSample(dest, capacity, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.CapacityBytes, nc.ts)
		_ = nc.check.QueueMetricSample(dest, free, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.AvailableBytes, nc.ts)
		_ = nc.check.QueueMetricSample(dest, used, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.UsedBytes, nc.ts)
	}
	{ // units:inodes
		streamTags := nc.check.NewTagList(parentStreamTags, []string{"units:inodes"})
		_ = nc.check.QueueMetricSample(dest, capacity, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.Inodes, nc.ts)
		_ = nc.check.QueueMetricSample(dest, free, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.InodesFree, nc.ts)
		_ = nc.check.QueueMetricSample(dest, used, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.InodesUsed, nc.ts)
	}
	{ // units:percent
		if stats.UsedBytes > 0 && stats.CapacityBytes > 0 {
			streamTags := nc.check.NewTagList(parentStreamTags, []string{"units:percent"})
			usedPct := ((float64(stats.UsedBytes) / float64(stats.CapacityBytes)) * 100)
			_ = nc.check.QueueMetricSample(dest, used, circonus.MetricTypeFloat64, streamTags, parentMeasurementTags, usedPct, nc.ts)
		}
	}
}

func (nc *Collector) queueFS(dest map[string]circonus.MetricSample, stats *fs, parentStreamTags []string, parentMeasurementTags []string) {
	baseStreamTags := nc.check.NewTagList(parentStreamTags, []string{"resource:fs"})
	nc.queueBaseFS(dest, stats, baseStreamTags, parentMeasurementTags)
}

func (nc *Collector) queueRuntimeImageFS(dest map[string]circonus.MetricSample, stats *fs, parentStreamTags []string, parentMeasurementTags []string) {
	baseStreamTags := nc.check.NewTagList(parentStreamTags, []string{"resource:runtime_image_fs"})
	nc.queueBaseFS(dest, stats, baseStreamTags, parentMeasurementTags)
}

func (nc *Collector) queueRootFS(dest map[string]circonus.MetricSample, stats *fs, parentStreamTags []string, parentMeasurementTags []string) {
	baseStreamTags := nc.check.NewTagList(parentStreamTags, []string{"resource:root_fs"})
	nc.queueBaseFS(dest, stats, baseStreamTags, parentMeasurementTags)
}

func (nc *Collector) queueLogsFS(dest map[string]circonus.MetricSample, stats *fs, parentStreamTags []string, parentMeasurementTags []string) {
	baseStreamTags := nc.check.NewTagList(parentStreamTags, []string{"resource:log_fs"})
	nc.queueBaseFS(dest, stats, baseStreamTags, parentMeasurementTags)
}

func (nc *Collector) queueEphemeralStorage(dest map[string]circonus.MetricSample, stats *fs, parentStreamTags []string, parentMeasurementTags []string) {
	baseStreamTags := nc.check.NewTagList(parentStreamTags, []string{"resource:ephemeral_storage"})
	nc.queueBaseFS(dest, stats, baseStreamTags, parentMeasurementTags)
}

func (nc *Collector) queueVolume(dest map[string]circonus.MetricSample, stats *volume, parentStreamTags []string, parentMeasurementTags []string) {
	baseStreamTags := nc.check.NewTagList(parentStreamTags, []string{"volume_name:" + stats.Name})
	nc.queueBaseFS(dest, &stats.fs, baseStreamTags, parentMeasurementTags)
}

func (nc *Collector) queueRlimit(dest map[string]circonus.MetricSample, stats *rlimit, parentStreamTags []string, parentMeasurementTags []string) {
	maxPID := "maxPID"
	curProc := "curProc"

	// units:procs
	streamTags := nc.check.NewTagList(parentStreamTags, []string{"resource:rlimit", "units:procs"})
	_ = nc.check.QueueMetricSample(dest, maxPID, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.MaxPID, nc.ts)
	_ = nc.check.QueueMetricSample(dest, curProc, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.CurProc, nc.ts)
}
