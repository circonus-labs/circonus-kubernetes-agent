// Copyright © 2019 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package collector

import (
	"io"

	"github.com/circonus-labs/circonus-kubernetes-agent/internal/circonus"
)

func (nc *Collector) queueCPU(dest map[string]circonus.MetricSample, stats *cpu, parentStreamTags []string, parentMeasurementTags []string) {
	cores := "usageNanoCores"
	seconds := "usageCoreNanoSeconds"

	var streamTags []string
	streamTags = append(streamTags, parentStreamTags...)
	streamTags = append(streamTags, "resource:cpu")
	_ = nc.check.QueueMetricSample(dest, cores, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.UsageNanoCores, nil)
	streamTags = append(streamTags, "units:seconds")
	_ = nc.check.QueueMetricSample(dest, seconds, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.UsageCoreNanoSeconds, nil)
}

func (nc *Collector) streamCPU(dest io.Writer, stats *cpu, parentStreamTags []string, parentMeasurementTags []string) {
	cores := "usageNanoCores"
	seconds := "usageCoreNanoSeconds"

	var streamTags []string
	streamTags = append(streamTags, parentStreamTags...)
	streamTags = append(streamTags, "resource:cpu")
	_ = nc.check.WriteMetricSample(dest, cores, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.UsageNanoCores, nc.ts)
	streamTags = append(streamTags, "units:seconds")
	_ = nc.check.WriteMetricSample(dest, seconds, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.UsageCoreNanoSeconds, nc.ts)
}

func (nc *Collector) queueMemory(dest map[string]circonus.MetricSample, stats *memory, parentStreamTags []string, parentMeasurementTags []string) {
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
		_ = nc.check.QueueMetricSample(dest, available, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.AvailableBytes, nil)
		_ = nc.check.QueueMetricSample(dest, used, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.UsageBytes, nil)
		_ = nc.check.QueueMetricSample(dest, workingSet, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.WorkingSetBytes, nil)
		_ = nc.check.QueueMetricSample(dest, rss, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.RSSBytes, nil)
	}
	{ // units:faults
		var streamTags []string
		streamTags = append(streamTags, parentStreamTags...)
		streamTags = append(streamTags, []string{"resource:memory", "units:faults"}...)
		_ = nc.check.QueueMetricSample(dest, pageFaults, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.PageFaults, nil)
		_ = nc.check.QueueMetricSample(dest, majorPageFaults, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.MajorPageFaults, nil)
	}
}

func (nc *Collector) streamMemory(dest io.Writer, stats *memory, parentStreamTags []string, parentMeasurementTags []string) {
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
		_ = nc.check.WriteMetricSample(dest, available, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.AvailableBytes, nc.ts)
		_ = nc.check.WriteMetricSample(dest, used, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.UsageBytes, nc.ts)
		_ = nc.check.WriteMetricSample(dest, workingSet, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.WorkingSetBytes, nc.ts)
		_ = nc.check.WriteMetricSample(dest, rss, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.RSSBytes, nc.ts)
	}
	{ // units:faults
		var streamTags []string
		streamTags = append(streamTags, parentStreamTags...)
		streamTags = append(streamTags, []string{"resource:memory", "units:faults"}...)
		_ = nc.check.WriteMetricSample(dest, pageFaults, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.PageFaults, nc.ts)
		_ = nc.check.WriteMetricSample(dest, majorPageFaults, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.MajorPageFaults, nc.ts)
	}
}

func (nc *Collector) queueNetwork(dest map[string]circonus.MetricSample, stats *network, parentStreamTags []string, parentMeasurementTags []string) {
	receive := "rx"
	transmit := "tx"

	{ // units:bytes
		var streamTags []string
		streamTags = append(streamTags, parentStreamTags...)
		streamTags = append(streamTags, []string{"resource:network", "units:bytes"}...)
		_ = nc.check.QueueMetricSample(dest, receive, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.RxBytes, nil)
		_ = nc.check.QueueMetricSample(dest, transmit, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.TxBytes, nil)
	}
	{ // units:errors
		var streamTags []string
		streamTags = append(streamTags, parentStreamTags...)
		streamTags = append(streamTags, []string{"resource:network", "units:errors"}...)
		_ = nc.check.QueueMetricSample(dest, receive, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.RxErrors, nil)
		_ = nc.check.QueueMetricSample(dest, transmit, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.TxErrors, nil)
	}

	for _, iface := range stats.Interfaces {
		{ // units:bytes
			var streamTags []string
			streamTags = append(streamTags, parentStreamTags...)
			streamTags = append(streamTags, []string{"resource:network", "units:bytes", "interface:" + iface.Name}...)
			_ = nc.check.QueueMetricSample(dest, receive, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, iface.RxBytes, nil)
			_ = nc.check.QueueMetricSample(dest, transmit, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, iface.TxBytes, nil)
		}
		{ // units:errors
			var streamTags []string
			streamTags = append(streamTags, parentStreamTags...)
			streamTags = append(streamTags, []string{"resource:network", "units:errors", "interface:" + iface.Name}...)
			_ = nc.check.QueueMetricSample(dest, receive, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, iface.RxErrors, nil)
			_ = nc.check.QueueMetricSample(dest, transmit, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, iface.TxErrors, nil)
		}
	}
}

func (nc *Collector) streamNetwork(dest io.Writer, stats *network, parentStreamTags []string, parentMeasurementTags []string) {
	receive := "rx"
	transmit := "tx"

	{ // units:bytes
		var streamTags []string
		streamTags = append(streamTags, parentStreamTags...)
		streamTags = append(streamTags, []string{"resource:network", "units:bytes"}...)
		_ = nc.check.WriteMetricSample(dest, receive, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.RxBytes, nc.ts)
		_ = nc.check.WriteMetricSample(dest, transmit, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.TxBytes, nc.ts)
	}
	{ // units:errors
		var streamTags []string
		streamTags = append(streamTags, parentStreamTags...)
		streamTags = append(streamTags, []string{"resource:network", "units:errors"}...)
		_ = nc.check.WriteMetricSample(dest, receive, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.RxErrors, nc.ts)
		_ = nc.check.WriteMetricSample(dest, transmit, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.TxErrors, nc.ts)
	}
	for _, iface := range stats.Interfaces {
		{ // units:bytes
			var streamTags []string
			streamTags = append(streamTags, parentStreamTags...)
			streamTags = append(streamTags, []string{"resource:network", "units:bytes", "interface:" + iface.Name}...)
			_ = nc.check.WriteMetricSample(dest, receive, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, iface.RxBytes, nil)
			_ = nc.check.WriteMetricSample(dest, transmit, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, iface.TxBytes, nil)
		}
		{ // units:errors
			var streamTags []string
			streamTags = append(streamTags, parentStreamTags...)
			streamTags = append(streamTags, []string{"resource:network", "units:errors", "interface:" + iface.Name}...)
			_ = nc.check.WriteMetricSample(dest, receive, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, iface.RxErrors, nil)
			_ = nc.check.WriteMetricSample(dest, transmit, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, iface.TxErrors, nil)
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
		_ = nc.check.QueueMetricSample(dest, capacity, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.CapacityBytes, nil)
		_ = nc.check.QueueMetricSample(dest, free, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.AvailableBytes, nil)
		_ = nc.check.QueueMetricSample(dest, used, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.UsedBytes, nil)
	}
	{ // units:inodes
		var streamTags []string
		streamTags = append(streamTags, parentStreamTags...)
		streamTags = append(streamTags, "units:inodes")
		_ = nc.check.QueueMetricSample(dest, capacity, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.Inodes, nil)
		_ = nc.check.QueueMetricSample(dest, free, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.InodesFree, nil)
		_ = nc.check.QueueMetricSample(dest, used, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.InodesUsed, nil)
	}
}

func (nc *Collector) streamBaseFS(dest io.Writer, stats *fs, parentStreamTags []string, parentMeasurementTags []string) {
	capacity := "capacity"
	free := "free"
	used := "used"

	{ // units:bytes
		var streamTags []string
		streamTags = append(streamTags, parentStreamTags...)
		streamTags = append(streamTags, "units:bytes")
		_ = nc.check.WriteMetricSample(dest, capacity, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.CapacityBytes, nc.ts)
		_ = nc.check.WriteMetricSample(dest, free, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.AvailableBytes, nc.ts)
		_ = nc.check.WriteMetricSample(dest, used, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.UsedBytes, nc.ts)
	}
	{ // units:inodes
		var streamTags []string
		streamTags = append(streamTags, parentStreamTags...)
		streamTags = append(streamTags, "units:inodes")
		_ = nc.check.WriteMetricSample(dest, capacity, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.Inodes, nc.ts)
		_ = nc.check.WriteMetricSample(dest, free, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.InodesFree, nc.ts)
		_ = nc.check.WriteMetricSample(dest, used, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.InodesUsed, nc.ts)
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

func (nc *Collector) streamFS(dest io.Writer, stats *fs, parentStreamTags []string, parentMeasurementTags []string) {
	var baseStreamTags []string
	if len(parentStreamTags) > 0 {
		baseStreamTags = make([]string, len(parentStreamTags))
		copy(baseStreamTags, parentStreamTags)
	}
	baseStreamTags = append(baseStreamTags, "resource:fs")
	nc.streamBaseFS(dest, stats, baseStreamTags, parentMeasurementTags)
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

func (nc *Collector) streamRuntimeImageFS(dest io.Writer, stats *fs, parentStreamTags []string, parentMeasurementTags []string) {
	var baseStreamTags []string
	if len(parentStreamTags) > 0 {
		baseStreamTags = make([]string, len(parentStreamTags))
		copy(baseStreamTags, parentStreamTags)
	}
	baseStreamTags = append(baseStreamTags, "resource:runtime_image_fs")
	nc.streamBaseFS(dest, stats, baseStreamTags, parentMeasurementTags)
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

func (nc *Collector) streamRootFS(dest io.Writer, stats *fs, parentStreamTags []string, parentMeasurementTags []string) {
	var baseStreamTags []string
	if len(parentStreamTags) > 0 {
		baseStreamTags = make([]string, len(parentStreamTags))
		copy(baseStreamTags, parentStreamTags)
	}
	baseStreamTags = append(baseStreamTags, "resource:root_fs")
	nc.streamBaseFS(dest, stats, baseStreamTags, parentMeasurementTags)
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

func (nc *Collector) streamLogsFS(dest io.Writer, stats *fs, parentStreamTags []string, parentMeasurementTags []string) {
	var baseStreamTags []string
	if len(parentStreamTags) > 0 {
		baseStreamTags = make([]string, len(parentStreamTags))
		copy(baseStreamTags, parentStreamTags)
	}
	baseStreamTags = append(baseStreamTags, "resource:logs_fs")
	nc.streamBaseFS(dest, stats, baseStreamTags, parentMeasurementTags)
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

func (nc *Collector) streamVolume(dest io.Writer, stats *volume, parentStreamTags []string, parentMeasurementTags []string) {
	var baseStreamTags []string
	if len(parentStreamTags) > 0 {
		baseStreamTags = make([]string, len(parentStreamTags))
		copy(baseStreamTags, parentStreamTags)
	}
	baseStreamTags = append(baseStreamTags, "volume_name:"+stats.Name)
	nc.streamBaseFS(dest, &stats.fs, baseStreamTags, parentMeasurementTags)
}

func (nc *Collector) queueRlimit(dest map[string]circonus.MetricSample, stats *rlimit, parentStreamTags []string, parentMeasurementTags []string) {
	maxPID := "maxPID"
	curProc := "curProc"

	// units:procs
	var streamTags []string
	streamTags = append(streamTags, parentStreamTags...)
	streamTags = append(streamTags, []string{"resource:rlimit", "units:procs"}...)
	_ = nc.check.QueueMetricSample(dest, maxPID, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.MaxPID, nil)
	_ = nc.check.QueueMetricSample(dest, curProc, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.CurProc, nil)
}

func (nc *Collector) streamRlimit(dest io.Writer, stats *rlimit, parentStreamTags []string, parentMeasurementTags []string) {
	maxPID := "maxPID"
	curProc := "curProc"

	// units:procs
	var streamTags []string
	streamTags = append(streamTags, parentStreamTags...)
	streamTags = append(streamTags, []string{"resource:rlimit", "units:procs"}...)
	_ = nc.check.WriteMetricSample(dest, maxPID, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.MaxPID, nc.ts)
	_ = nc.check.WriteMetricSample(dest, curProc, circonus.MetricTypeUint64, streamTags, parentMeasurementTags, stats.CurProc, nc.ts)
}
