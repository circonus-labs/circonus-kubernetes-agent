// Copyright © 2019 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package circonus

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"

	cgm "github.com/circonus-labs/circonus-gometrics/v3"
)

const (
	// MetricTypeInt32 reconnoiter
	MetricTypeInt32 = "i"

	// MetricTypeUint32 reconnoiter
	MetricTypeUint32 = "I"

	// MetricTypeInt64 reconnoiter
	MetricTypeInt64 = "l"

	// MetricTypeUint64 reconnoiter
	MetricTypeUint64 = "L"

	// MetricTypeFloat64 reconnoiter
	MetricTypeFloat64 = "n"

	// MetricTypeString reconnoiter
	MetricTypeString = "s"

	// MetricTypeHistogram reconnoiter
	MetricTypeHistogram = "h"

	// MetricTypeCumulativeHistogram reconnoiter
	MetricTypeCumulativeHistogram = "H"

	// NOTE: max tags and metric name len are enforced here so that
	// details on which metric(s) can be logged. Otherwise, any
	// metric(s) exceeding the limits are rejected by the broker
	// without details on exactly which metric(s) caused the error.
	// All metrics sent with the offending metric(s) are also rejected.

	MaxTagLen = 256 // sync w/NOIT_TAG_MAX_PAIR_LEN https://github.com/circonus-labs/reconnoiter/blob/master/src/noit_metric.h#L102
	MaxTagCat = 254 // sync w/NOIT_TAG_MAX_CAT_LEN https://github.com/circonus-labs/reconnoiter/blob/master/src/noit_metric.h#L104

	// MaxTags reconnoiter will accept in stream tagged metric name
	MaxTags = 256 // sync w/MAX_TAGS https://github.com/circonus-labs/reconnoiter/blob/master/src/noit_metric.h#L46

	// MaxMetricNameLen reconnoiter will accept (name+stream tags)
	MaxMetricNameLen = 4096 // sync w/MAX_METRIC_TAGGED_NAME https://github.com/circonus-labs/reconnoiter/blob/master/src/noit_metric.h#L45
)

type Metric struct {
	Name  string
	Value MetricSample
}

type MetricSample struct {
	Value     interface{} `json:"_value"`
	Type      string      `json:"_type"`
	Timestamp uint64      `json:"_ts,omitempty"`
}

var metricTypeRx = regexp.MustCompile(`^[` + strings.Join([]string{
	MetricTypeInt32,
	MetricTypeUint32,
	MetricTypeInt64,
	MetricTypeUint64,
	MetricTypeFloat64,
	MetricTypeString,
	MetricTypeHistogram,
	MetricTypeCumulativeHistogram,
}, "") + `]$`)

// AddGauge to queue for submission
func (c *Check) AddGauge(metricName string, tags cgm.Tags, value interface{}) {
	c.metricsmu.Lock()
	defer c.metricsmu.Unlock()
	if c.metrics != nil {
		tags = append(tags, c.defaultTags...)
		c.metrics.GaugeWithTags(metricName, tags, value)
	}
}

// AddHistSample to queue for submission
func (c *Check) AddHistSample(metricName string, tags cgm.Tags, value float64) {
	c.metricsmu.Lock()
	defer c.metricsmu.Unlock()
	if c.metrics != nil {
		tags = append(tags, c.defaultTags...)
		c.metrics.TimingWithTags(metricName, tags, value)
	}
}

// AddText to queue for submission
func (c *Check) AddText(metricName string, tags cgm.Tags, value string) {
	c.metricsmu.Lock()
	defer c.metricsmu.Unlock()
	if c.metrics != nil {
		tags = append(tags, c.defaultTags...)
		c.metrics.SetTextWithTags(metricName, tags, value)
	}
}

// IncrementCounter to queue for submission
func (c *Check) IncrementCounter(metricName string, tags cgm.Tags) {
	c.metricsmu.Lock()
	defer c.metricsmu.Unlock()
	if c.metrics != nil {
		tags = append(tags, c.defaultTags...)
		c.metrics.IncrementWithTags(metricName, tags)
	}
}

// IncrementCounterByValue to queue for submission
func (c *Check) IncrementCounterByValue(metricName string, tags cgm.Tags, val uint64) {
	c.metricsmu.Lock()
	defer c.metricsmu.Unlock()
	if c.metrics != nil {
		tags = append(tags, c.defaultTags...)
		c.metrics.IncrementByValueWithTags(metricName, tags, val)
	}
}

// SetCounter to queue for submission
func (c *Check) SetCounter(metricName string, tags cgm.Tags, value uint64) {
	c.metricsmu.Lock()
	defer c.metricsmu.Unlock()
	if c.metrics != nil {
		tags = append(tags, c.defaultTags...)
		c.metrics.SetWithTags(metricName, tags, value)
	}
}

// QueueMetricSample to queue for submission
func (c *Check) QueueMetricSample(
	metrics map[string]MetricSample,
	metricName,
	metricType string,
	streamTags,
	measurementTags []string,
	value interface{},
	timestamp *time.Time,
) error {
	if metrics == nil {
		return errors.New("invalid metrics queue (nil)")
	}
	if metricName == "" {
		return errors.New("invalid metric name (empty)")
	}
	if metricType == "" {
		return errors.New("invalid metric type (empty)")
	}

	applyFilters := true
	if strings.Contains(strings.Join(streamTags, ","), "collector:dynamic") {
		applyFilters = c.filterDynamicMetrics
	}

	if applyFilters && len(c.metricFilters) > 0 {
		rejectMetric := true
		origName := metricName
		if strings.Contains(metricName, "|ST") {
			parts := strings.SplitN(metricName, "|", 2)
			if len(parts) == 2 {
				metricName = parts[0]
			}
		}
		for _, mf := range c.metricFilters {
			if !mf.Enabled {
				continue
			}
			if mf.Filter.MatchString(metricName) {
				if mf.Allow {
					rejectMetric = false
					break
				}
			}
		}
		metricName = origName
		if rejectMetric {
			c.statsmu.Lock()
			c.stats.LocFiltered++
			c.statsmu.Unlock()
			return nil
		}
	}

	streamTagList := c.NewTagList(streamTags, strings.Split(c.config.DefaultStreamtags, ","))

	if len(streamTagList)+len(measurementTags) > MaxTags {
		c.log.Warn().
			Str("metric_name", metricName).
			Strs("stream_tags", streamTagList).
			Strs("measurement_tags", measurementTags).
			Int("num_tags", len(streamTagList)+len(measurementTags)).
			Int("max_tags", MaxTags).
			Msg("max metric tags exceeded, discarding")
		return nil
	}

	taggedMetricName := c.taggedName(metricName, streamTagList, measurementTags)

	if len(taggedMetricName) > MaxMetricNameLen {
		c.log.Warn().
			Str("metric_name", taggedMetricName).
			Int("encoded_len", len(taggedMetricName)).
			Int("max_len", MaxMetricNameLen).
			Msg("max metric name length exceeded, discarding")
		return nil
	}

	if !metricTypeRx.MatchString(metricType) {
		return fmt.Errorf("unrecognized circonus metric type (%s)", metricType)
	}

	if metricType == MetricTypeFloat64 && math.IsNaN(value.(float64)) {
		c.log.Warn().
			Str("metric_name", metricName).
			Str("metric_type", metricType).
			Interface("metric_value", value).
			Strs("stream_tags", streamTagList).
			Msg("is NaN, skipping")
		return fmt.Errorf("metric value is NaN")
	}

	val := value
	if metricType == MetricTypeString {
		val = value.(string)
	}

	if _, found := metrics[taggedMetricName]; found {
		c.log.Warn().
			Str("metric_name", metricName).
			Strs("stream_tags", streamTagList).
			Strs("measurement_tags", measurementTags).
			Str("tagged_name", taggedMetricName).
			Msg("already present, overwriting...")
	}

	metricSample := MetricSample{
		Type:  metricType,
		Value: val,
	}

	if timestamp != nil && (metricType != MetricTypeHistogram && metricType != MetricTypeCumulativeHistogram) {
		metricSample.Timestamp = makeTimestamp(timestamp)
	}

	metrics[taggedMetricName] = metricSample

	return nil
}

// makeTimestamp returns timestamp in ms units for _ts metric value
func makeTimestamp(ts *time.Time) uint64 {
	return uint64(ts.UTC().UnixNano() / (int64(time.Millisecond) / int64(time.Nanosecond)))
}
