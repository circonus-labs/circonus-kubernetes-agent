// Copyright © 2020 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package circonus

import "code.cloudfoundry.org/bytefmt"

// Stats defines the submission stats tracked across metric submissions to broker
type Stats struct {
	Transmitted string
	LocFiltered uint64 // agent: filtered based on namerx
	BkrFiltered uint64 // broker: filtered
	RecvMetrics uint64 // broker: "stats" received
	SentMetrics uint64 // agent: total "unique" metrics sent
	SentBytes   uint64 // size of raw data (not compressed)
	SentSize    uint64 // actual size of data sent
}

// SubmitStats returns copy of the submission stats
func (c *Check) SubmitStats() Stats {
	c.statsmu.Lock()
	defer c.statsmu.Unlock()
	return Stats{
		LocFiltered: c.stats.LocFiltered,
		BkrFiltered: c.stats.BkrFiltered,
		RecvMetrics: c.stats.RecvMetrics,
		SentMetrics: c.stats.SentMetrics,
		SentBytes:   c.stats.SentBytes,
		SentSize:    c.stats.SentSize,
		Transmitted: bytefmt.ByteSize(c.stats.SentSize),
	}
}

// ResetSubmitStats zeros submission stats
func (c *Check) ResetSubmitStats() {
	c.statsmu.Lock()
	defer c.statsmu.Unlock()
	c.stats.SentMetrics = 0
	c.stats.RecvMetrics = 0
	c.stats.SentBytes = 0
	c.stats.SentSize = 0
	c.stats.LocFiltered = 0
	c.stats.BkrFiltered = 0
}
