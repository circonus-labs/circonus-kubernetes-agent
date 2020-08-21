// Copyright © 2020 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package circonus

import "code.cloudfoundry.org/bytefmt"

// Stats defines the submission stats tracked across metric submissions to broker
type Stats struct {
	Filtered  uint64 // agent: filtered based on namerx
	BFiltered uint64 // broker: filtered
	Metrics   uint64 // broker: "stats" received
	TMetrics  uint64 // agent: total "unique" metrics sent
	SentBytes uint64
	SentSize  string
}

// SubmitStats returns copy of the submission stats
func (c *Check) SubmitStats() Stats {
	c.statsmu.Lock()
	defer c.statsmu.Unlock()
	return Stats{
		Filtered:  c.stats.Filtered,
		BFiltered: c.stats.BFiltered,
		Metrics:   c.stats.Metrics,
		TMetrics:  c.stats.TMetrics,
		SentBytes: c.stats.SentBytes,
		SentSize:  bytefmt.ByteSize(c.stats.SentBytes),
	}
}

// ResetSubmitStats zeros submission stats
func (c *Check) ResetSubmitStats() {
	c.statsmu.Lock()
	defer c.statsmu.Unlock()
	c.stats.Metrics = 0
	c.stats.TMetrics = 0
	c.stats.SentBytes = 0
	c.stats.Filtered = 0
	c.stats.BFiltered = 0
}
