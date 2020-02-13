// Copyright © 2019 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package circonus

import (
	"errors"
	"fmt"
	"io"
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

	// MaxTags reconnoiter will accept in stream tagged metric name
	MaxTags = 256 // sync w/MAX_TAGS https://github.com/circonus-labs/reconnoiter/blob/master/src/noit_metric.h#L41

	// MaxMetricNameLen reconnoiter will accept (name+stream tags)
	MaxMetricNameLen = 4096 // sync w/MAX_METRIC_TAGGED_NAME https://github.com/circonus-labs/reconnoiter/blob/master/src/noit_metric.h#L40
)

type MetricSample struct {
	Value     interface{} `json:"_value"`
	Type      string      `json:"_type"`
	Timestamp uint64      `json:"_ts,omitempty"`
}

var (
	metricTypeRx = regexp.MustCompile(`^[` + strings.Join([]string{
		MetricTypeInt32,
		MetricTypeUint32,
		MetricTypeInt64,
		MetricTypeUint64,
		MetricTypeFloat64,
		MetricTypeString,
		MetricTypeHistogram,
		MetricTypeCumulativeHistogram,
	}, "") + `]$`)
)

// AddGauge to queue for submission
func (c *Check) AddGauge(metricName string, tags cgm.Tags, value interface{}) {
	if c.metrics != nil {
		c.metrics.GaugeWithTags(metricName, tags, value)
	}
}

// AddHistSample to queue for submission
func (c *Check) AddHistSample(metricName string, tags cgm.Tags, value float64) {
	if c.metrics != nil {
		c.metrics.TimingWithTags(metricName, tags, value)
	}
}

// AddText to queue for submission
func (c *Check) AddText(metricName string, tags cgm.Tags, value string) {
	if c.metrics != nil {
		c.metrics.SetTextWithTags(metricName, tags, value)
	}
}

// WriteMetricSample to queue for submission
func (c *Check) WriteMetricSample(
	metricDest io.Writer,
	metricName,
	metricType string,
	streamTags,
	measurementTags []string,
	value interface{},
	timestamp *time.Time) error {

	if metricDest == nil {
		return errors.New("invalid metric destination (nil)")
	}
	if metricName == "" {
		return errors.New("invalid metric name (empty)")
	}
	if metricType == "" {
		return errors.New("invalid metric type (empty)")
	}

	streamTagList := strings.Split(c.config.DefaultStreamtags, ",")
	streamTagList = append(streamTagList, streamTags...)

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

	val := value
	if metricType == "s" {
		val = fmt.Sprintf("%q", value.(string))
	}

	var metricSample string
	if timestamp != nil {
		metricSample = fmt.Sprintf(
			`{"%s":{"_type":"%s","_value":%v,"_ts":%d}}`,
			taggedMetricName,
			metricType,
			val,
			makeTimestamp(timestamp),
		)
	} else {
		metricSample = fmt.Sprintf(
			`{"%s":{"_type":"%s","_value":%v}}`,
			taggedMetricName,
			metricType,
			val,
		)
	}

	_, err := fmt.Fprintln(metricDest, metricSample)
	if err != nil {
		return err
	}
	return nil
}

// QueueMetricSample to queue for submission
func (c *Check) QueueMetricSample(
	metrics map[string]MetricSample,
	metricName,
	metricType string,
	streamTags,
	measurementTags []string,
	value interface{},
	timestamp *time.Time) error {

	if metrics == nil {
		return errors.New("invalid metrics queue (nil)")
	}
	if metricName == "" {
		return errors.New("invalid metric name (empty)")
	}
	if metricType == "" {
		return errors.New("invalid metric type (empty)")
	}

	streamTagList := strings.Split(c.config.DefaultStreamtags, ",")
	streamTagList = append(streamTagList, streamTags...)

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

	val := value
	if metricType == "s" {
		val = fmt.Sprintf("%q", value.(string))
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

	if timestamp != nil {
		metricSample.Timestamp = makeTimestamp(timestamp)
	}

	metrics[taggedMetricName] = metricSample

	return nil
}

// makeTimestamp returns timestamp in ms units for _ts metric value
func makeTimestamp(ts *time.Time) uint64 {
	return uint64(ts.UTC().UnixNano() / (int64(time.Millisecond) / int64(time.Nanosecond)))
}
