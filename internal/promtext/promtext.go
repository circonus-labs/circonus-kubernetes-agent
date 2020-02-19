// Copyright © 2019 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// Package promtext parses prometheus text metrics
package promtext

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"strings"
	"time"

	"github.com/circonus-labs/circonus-kubernetes-agent/internal/circonus"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/rs/zerolog"
)

const (
	// NOTE: these will be enabled in a future release when we support cumulative histograms (type H)
	//       flip emitHistogramBuckets to true and leave circCumulativeHistogram true. Once we're
	//       happy with the support the gating flag logic can be removed and the code simplifed.
	emitHistogramBuckets    = false
	circCumulativeHistogram = true
)

// QueueMetrics is a generic function to digest prometheus text format metrics and
// emit circonus formatted metrics.
// Formats supported: https://prometheus.io/docs/instrumenting/exposition_formats/
func QueueMetrics(
	ctx context.Context,
	check *circonus.Check,
	logger zerolog.Logger,
	data io.Reader,
	parentStreamTags []string,
	parentMeasurementTags []string,
	ts *time.Time) error {

	var baseStreamTags []string
	if len(parentStreamTags) > 0 {
		baseStreamTags = make([]string, len(parentStreamTags))
		copy(baseStreamTags, parentStreamTags)
	}

	var parser expfmt.TextParser

	metricFamilies, err := parser.TextToMetricFamilies(data)
	if err != nil {
		return err
	}

	metrics := make(map[string]circonus.MetricSample)
	maxMetrics := check.MaxMetricBucketSize()

	for mn, mf := range metricFamilies {
		if done(ctx) {
			return nil
		}
		for _, m := range mf.Metric {
			if maxMetrics > 0 && len(metrics) >= maxMetrics {
				if err := check.SubmitQueue(ctx, metrics, logger); err != nil {
					logger.Warn().Err(err).Msg("submitting metrics")
				}
				metrics = make(map[string]circonus.MetricSample)
			}
			if done(ctx) {
				return nil
			}
			metricName := mn
			streamTags := getLabels(m)
			streamTags = append(streamTags, baseStreamTags...)
			switch mf.GetType() {
			case dto.MetricType_SUMMARY:
				_ = check.QueueMetricSample(
					metrics, metricName+"_count",
					circonus.MetricTypeUint64,
					streamTags, parentMeasurementTags,
					m.GetSummary().GetSampleCount(), ts)
				_ = check.QueueMetricSample(
					metrics, metricName+"_sum",
					circonus.MetricTypeFloat64,
					streamTags, parentMeasurementTags,
					m.GetSummary().GetSampleSum(), ts)
				for qn, qv := range getQuantiles(m) {
					var qtags []string
					qtags = append(qtags, streamTags...)
					qtags = append(qtags, "quantile:"+qn)
					_ = check.QueueMetricSample(
						metrics, metricName,
						circonus.MetricTypeFloat64,
						qtags, parentMeasurementTags,
						qv, nil)
				}
			case dto.MetricType_HISTOGRAM:
				_ = check.QueueMetricSample(
					metrics, metricName+"_count",
					circonus.MetricTypeUint64,
					streamTags, parentMeasurementTags,
					m.GetHistogram().GetSampleCount(), ts)
				_ = check.QueueMetricSample(
					metrics, metricName+"_sum",
					circonus.MetricTypeFloat64,
					streamTags, parentMeasurementTags,
					m.GetHistogram().GetSampleSum(), ts)
				if emitHistogramBuckets {
					if circCumulativeHistogram {
						var htags []string
						htags = append(htags, streamTags...)
						histo := promHistoBucketsToCircHisto(m)
						if len(histo) > 0 {
							_ = check.QueueMetricSample(
								metrics, metricName,
								circonus.MetricTypeCumulativeHistogram,
								htags, parentMeasurementTags,
								strings.Join(histo, ","), ts)
						}
					} else {
						for bn, bv := range getBuckets(m) {
							var htags []string
							htags = append(htags, streamTags...)
							htags = append(htags, "bucket:"+bn)
							_ = check.QueueMetricSample(
								metrics, metricName,
								circonus.MetricTypeUint64,
								htags, parentMeasurementTags,
								bv, ts)
						}
					}
				}
			default:
				switch {
				case m.Gauge != nil:
					if m.GetGauge().Value != nil {
						_ = check.QueueMetricSample(
							metrics, metricName,
							circonus.MetricTypeFloat64,
							streamTags, parentMeasurementTags,
							*m.GetGauge().Value, ts)
					}
				case m.Counter != nil:
					if m.GetCounter().Value != nil {
						_ = check.QueueMetricSample(
							metrics, metricName,
							circonus.MetricTypeFloat64,
							streamTags, parentMeasurementTags,
							*m.GetCounter().Value, ts)
					}
				case m.Untyped != nil:
					if m.GetUntyped().Value != nil {
						if *m.GetUntyped().Value == math.Inf(+1) {
							logger.Warn().
								Str("metric", metricName).
								Str("type", mf.GetType().String()).
								Str("value", (*m).GetUntyped().String()).
								Msg("cannot coerce +Inf to uint64")
							continue
						}
						_ = check.QueueMetricSample(
							metrics, metricName,
							circonus.MetricTypeFloat64,
							streamTags, parentMeasurementTags,
							*m.GetUntyped().Value, ts)
					}
				}
			}
		}
	}

	// send any remaining metrics
	if len(metrics) > 0 {
		if err := check.SubmitQueue(ctx, metrics, logger); err != nil {
			logger.Warn().Err(err).Msg("submitting metrics")
		}
	}

	return nil
}

// StreamMetrics is a generic function to digest prometheus text format metrics and
// emit circonus formatted metrics.
// Formats supported: https://prometheus.io/docs/instrumenting/exposition_formats/
func StreamMetrics(
	ctx context.Context,
	check *circonus.Check,
	logger zerolog.Logger,
	data io.Reader,
	parentStreamTags []string,
	parentMeasurementTags []string,
	ts *time.Time) error {

	var baseStreamTags []string
	if len(parentStreamTags) > 0 {
		baseStreamTags = make([]string, len(parentStreamTags))
		copy(baseStreamTags, parentStreamTags)
	}

	var parser expfmt.TextParser

	metricFamilies, err := parser.TextToMetricFamilies(data)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	metricsQueued := 0
	maxMetrics := check.MaxMetricBucketSize()

	for mn, mf := range metricFamilies {
		if done(ctx) {
			return nil
		}
		for _, m := range mf.Metric {
			if done(ctx) {
				return nil
			}
			if maxMetrics > 0 && metricsQueued >= maxMetrics {
				if err := check.SubmitStream(ctx, &buf, logger); err != nil {
					logger.Warn().Err(err).Msg("submitting metrics")
				}
				buf.Reset()
			}
			metricName := mn
			streamTags := getLabels(m)
			streamTags = append(streamTags, baseStreamTags...)
			switch mf.GetType() {
			case dto.MetricType_SUMMARY:
				_ = check.WriteMetricSample(
					&buf, metricName+"_count",
					circonus.MetricTypeUint64,
					streamTags, parentMeasurementTags,
					m.GetSummary().GetSampleCount(), ts)
				metricsQueued++
				_ = check.WriteMetricSample(
					&buf, metricName+"_sum",
					circonus.MetricTypeFloat64,
					streamTags, parentMeasurementTags,
					m.GetSummary().GetSampleSum(), ts)
				metricsQueued++
				for qn, qv := range getQuantiles(m) {
					var qtags []string
					qtags = append(qtags, streamTags...)
					qtags = append(qtags, "quantile:"+qn)
					_ = check.WriteMetricSample(
						&buf, metricName,
						circonus.MetricTypeFloat64,
						qtags, parentMeasurementTags,
						qv, nil)
					metricsQueued++
				}
			case dto.MetricType_HISTOGRAM:
				_ = check.WriteMetricSample(
					&buf, metricName+"_count",
					circonus.MetricTypeUint64,
					streamTags, parentMeasurementTags,
					m.GetHistogram().GetSampleCount(), ts)
				metricsQueued++
				_ = check.WriteMetricSample(
					&buf, metricName+"_sum",
					circonus.MetricTypeFloat64,
					streamTags, parentMeasurementTags,
					m.GetHistogram().GetSampleSum(), ts)
				metricsQueued++
				if emitHistogramBuckets {
					if circCumulativeHistogram {
						var htags []string
						htags = append(htags, streamTags...)
						histo := promHistoBucketsToCircHisto(m)
						if len(histo) > 0 {
							_ = check.WriteMetricSample(
								&buf, metricName,
								circonus.MetricTypeCumulativeHistogram,
								htags, parentMeasurementTags,
								strings.Join(histo, ","), ts)
							metricsQueued++
						}
					} else {
						for bn, bv := range getBuckets(m) {
							var htags []string
							htags = append(htags, streamTags...)
							htags = append(htags, "bucket:"+bn)
							_ = check.WriteMetricSample(
								&buf, metricName,
								circonus.MetricTypeUint64,
								htags, parentMeasurementTags,
								bv, ts)
							metricsQueued++
						}
					}
				}
			default:
				switch {
				case m.Gauge != nil:
					if m.GetGauge().Value != nil {
						_ = check.WriteMetricSample(
							&buf, metricName,
							circonus.MetricTypeFloat64,
							streamTags, parentMeasurementTags,
							*m.GetGauge().Value, ts)
						metricsQueued++
					}
				case m.Counter != nil:
					if m.GetCounter().Value != nil {
						_ = check.WriteMetricSample(
							&buf, metricName,
							circonus.MetricTypeFloat64,
							streamTags, parentMeasurementTags,
							*m.GetCounter().Value, ts)
						metricsQueued++
					}
				case m.Untyped != nil:
					if m.GetUntyped().Value != nil {
						if *m.GetUntyped().Value == math.Inf(+1) {
							logger.Warn().
								Str("metric", metricName).
								Str("type", mf.GetType().String()).
								Str("value", (*m).GetUntyped().String()).
								Msg("cannot coerce +Inf to uint64")
							continue
						}
						_ = check.WriteMetricSample(
							&buf, metricName,
							circonus.MetricTypeFloat64,
							streamTags, parentMeasurementTags,
							*m.GetUntyped().Value, ts)
						metricsQueued++
					}
				}
			}
		}
	}
	// send any remaining metrics
	if buf.Len() > 0 {
		if err := check.SubmitStream(ctx, &buf, logger); err != nil {
			logger.Warn().Err(err).Msg("submitting metrics")
		}

	}

	return nil
}

func getLabels(m *dto.Metric) []string {
	labels := []string{}

	for _, label := range m.Label {
		if label.Name != nil && label.Value != nil {
			labels = append(labels, *label.Name+":"+*label.Value)
		}
	}

	return labels
}

func getQuantiles(m *dto.Metric) map[string]float64 {
	ret := make(map[string]float64)
	for _, q := range m.GetSummary().Quantile {
		if q.Value != nil && !math.IsNaN(*q.Value) {
			ret[fmt.Sprint(*q.Quantile)] = *q.Value
		}
	}
	return ret
}

func getBuckets(m *dto.Metric) map[string]uint64 {
	ret := make(map[string]uint64)
	for _, b := range m.GetHistogram().Bucket {
		if b.CumulativeCount != nil {
			ret[fmt.Sprint(*b.UpperBound)] = *b.CumulativeCount
		}
	}
	return ret
}

func promHistoBucketsToCircHisto(m *dto.Metric) []string {
	const reducer = 0.999
	var ret []string
	tot := m.GetHistogram().GetSampleCount()
	n := uint64(0)
	for _, b := range m.GetHistogram().Bucket {
		if b.CumulativeCount != nil {
			v := *b.CumulativeCount - n
			if v > 0 {
				upperBound := *b.UpperBound
				if upperBound == math.Inf(+1) {
					upperBound = 10e+127
				} else {
					upperBound *= reducer
				}
				ret = append(ret, fmt.Sprintf("H[%e]=%d", upperBound, v))
				if *b.CumulativeCount == tot {
					break
				}
				n += v
			}
		}
	}
	return ret
}

func done(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}
