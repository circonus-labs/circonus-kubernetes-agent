// Copyright © 2019 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package circonus

import (
	"bytes"
	// "compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path"
	"strconv"
	"time"

	"code.cloudfoundry.org/bytefmt"
	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/release"
	"github.com/google/uuid"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/klauspost/compress/gzip"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

type TrapResult struct {
	CheckUUID  string
	SubmitUUID uuid.UUID
	Stats      uint64 `json:"stats"`
	Error      string `json:"error,omitempty"`
}

const (
	compressionThreshold = 1024
	traceTSFormat        = "20060102_150405.000000000"
)

func (c *Check) FlushCGM(ctx context.Context, ts *time.Time) {
	if c.metrics != nil {
		// TODO: add timestamp support to CGM (e.g. FlushMetricsWithTimestamp(ts))
		metrics := make(map[string]MetricSample)
		for mn, mv := range *(c.metrics.FlushMetrics()) {
			ms := MetricSample{
				Value: mv.Value,
				Type:  mv.Type,
			}
			if ms.Type != MetricTypeHistogram {
				ms.Timestamp = makeTimestamp(ts)
			}
			metrics[mn] = ms
		}

		if err := c.SubmitMetrics(ctx, metrics, c.log, false); err != nil {
			c.log.Error().Err(err).Msg("submitting cgm metrics")
		}
	}
}

func (c *Check) ResetSubmitStats() {
	c.statsmu.Lock()
	defer c.statsmu.Unlock()
	c.stats.Metrics = 0
	c.stats.SentBytes = 0
	c.stats.Filtered = 0
}

func (c *Check) SubmitStats() Stats {
	c.statsmu.Lock()
	defer c.statsmu.Unlock()
	return Stats{
		Filtered:  c.stats.Filtered,
		Metrics:   c.stats.Metrics,
		SentBytes: c.stats.SentBytes,
		SentSize:  bytefmt.ByteSize(c.stats.SentBytes),
	}
}

// Submit sends metrics to a circonus trap
func (c *Check) SubmitMetrics(ctx context.Context, metrics map[string]MetricSample, resultLogger zerolog.Logger, includeStats bool) error {
	if metrics == nil {
		return errors.New("invalid metrics (nil)")
	}
	if len(metrics) == 0 {
		return nil
	}

	rawData, err := json.Marshal(metrics)
	if err != nil {
		resultLogger.Error().Err(err).Msg("json encoding metrics")
		return errors.Wrap(err, "marshaling metrics")
	}

	start := time.Now()

	if c.submissionURL == "" {
		if c.config.DryRun {
			_, err := io.Copy(os.Stdout, bytes.NewReader(rawData))
			return err
		}
		return errors.New("no submission url and not in dry-run mode")
	}

	if c.client == nil {
		if c.brokerTLSConfig != nil {
			c.client = &http.Client{
				Transport: &http.Transport{
					Proxy: http.ProxyFromEnvironment,
					DialContext: (&net.Dialer{
						Timeout:       10 * time.Second,
						KeepAlive:     3 * time.Second,
						FallbackDelay: -1 * time.Millisecond,
					}).DialContext,
					TLSClientConfig:     c.brokerTLSConfig,
					TLSHandshakeTimeout: 10 * time.Second,
					DisableKeepAlives:   true,
					DisableCompression:  false,
					MaxIdleConns:        1,
					MaxIdleConnsPerHost: 0,
				},
			}
		} else {
			c.client = &http.Client{
				Transport: &http.Transport{
					Proxy: http.ProxyFromEnvironment,
					DialContext: (&net.Dialer{
						Timeout:       10 * time.Second,
						KeepAlive:     3 * time.Second,
						FallbackDelay: -1 * time.Millisecond,
					}).DialContext,
					DisableKeepAlives:   true,
					DisableCompression:  false,
					MaxIdleConns:        1,
					MaxIdleConnsPerHost: 0,
				},
			}
		}
	}

	submitUUID, err := uuid.NewRandom()
	if err != nil {
		resultLogger.Error().Err(err).Msg("creating new submit ID")
		return errors.Wrap(err, "creating new submit ID")
	}

	payloadIsCompressed := false

	var subData *bytes.Buffer
	if c.UseCompression() && len(rawData) > compressionThreshold {
		subData = bytes.NewBuffer([]byte{})
		zw := gzip.NewWriter(subData)
		n, e1 := zw.Write(rawData)
		if e1 != nil {
			resultLogger.Error().Err(e1).Msg("compressing metrics")
			return errors.Wrap(e1, "compressing metrics")
		}
		if n != len(rawData) {
			resultLogger.Error().Int("data_len", len(rawData)).Int("written", n).Msg("gzip write length mismatch")
			return errors.Errorf("write length mismatch data length %d != written length %d", len(rawData), n)
		}
		if e2 := zw.Close(); e2 != nil {
			resultLogger.Error().Err(e2).Msg("closing gzip writer")
			return errors.Wrap(e2, "closing gzip writer")
		}
		payloadIsCompressed = true
	} else {
		subData = bytes.NewBuffer(rawData)
	}

	if dumpDir := c.config.TraceSubmits; dumpDir != "" {
		fn := path.Join(dumpDir, time.Now().UTC().Format(traceTSFormat)+"_"+submitUUID.String()+".json")
		if payloadIsCompressed {
			fn += ".gz"
		}

		if fh, e1 := os.Create(fn); e1 != nil {
			c.log.Error().Err(e1).Str("file", fn).Msg("skipping submit trace")
		} else {
			if _, e2 := fh.Write(subData.Bytes()); e2 != nil {
				resultLogger.Error().Err(e2).Msg("writing metric trace")
			}
			if e3 := fh.Close(); e3 != nil {
				resultLogger.Error().Err(e3).Str("file", fn).Msg("closing metric trace")
			}
		}
	}

	dataLen := subData.Len()

	reqStart := time.Now()

	req, err := retryablehttp.NewRequest("PUT", c.submissionURL, subData)
	if err != nil {
		resultLogger.Error().Err(err).Msg("creating submission request")
		return err
	}
	req = req.WithContext(ctx)
	req.Header.Set("User-Agent", release.NAME+"/"+release.VERSION)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Connection", "close")
	req.Header.Set("Content-Length", strconv.Itoa(dataLen))
	if payloadIsCompressed {
		req.Header.Set("Content-Encoding", "gzip")
	}

	retryClient := retryablehttp.NewClient()
	retryClient.HTTPClient = c.client
	retryClient.Logger = logshim{logh: c.log.With().Str("pkg", "retryablehttp").Logger()}
	retryClient.RetryWaitMin = 50 * time.Millisecond
	retryClient.RetryWaitMax = 1 * time.Second
	retryClient.RetryMax = 10
	retryClient.RequestLogHook = func(l retryablehttp.Logger, r *http.Request, attempt int) {
		if attempt > 0 {
			c.metrics.IncrementWithTags("collect_submit_retries", cgm.Tags{cgm.Tag{Category: "source", Value: release.NAME}})
			reqStart = time.Now()
			resultLogger.Warn().Str("url", r.URL.String()).Int("retry", attempt).Msg("retrying...")
		}
	}
	retryClient.ResponseLogHook = func(l retryablehttp.Logger, r *http.Response) {
		c.AddHistSample("collect_latency", cgm.Tags{
			cgm.Tag{Category: "type", Value: "submit"},
			cgm.Tag{Category: "source", Value: release.NAME},
			cgm.Tag{Category: "units", Value: "milliseconds"},
		}, float64(time.Since(reqStart).Milliseconds()))
		if r.StatusCode != http.StatusOK {
			c.metrics.IncrementWithTags("collect_submit_errors", cgm.Tags{
				cgm.Tag{Category: "code", Value: fmt.Sprintf("%d", r.StatusCode)},
				cgm.Tag{Category: "source", Value: release.NAME},
			})
			resultLogger.Warn().Str("url", r.Request.URL.String()).Str("status", r.Status).Msg("non-200 response...")
		}
	}

	defer retryClient.HTTPClient.CloseIdleConnections()

	resp, err := retryClient.Do(req)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		resultLogger.Error().Err(err).Msg("making request")
		c.metrics.IncrementWithTags("collect_submit_fails", cgm.Tags{
			cgm.Tag{Category: "source", Value: release.NAME},
		})
		return err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		resultLogger.Error().Err(err).Msg("reading body")
		return err
	}

	if resp.StatusCode != http.StatusOK {
		c.metrics.IncrementWithTags("collect_submit_fails", cgm.Tags{
			cgm.Tag{Category: "code", Value: fmt.Sprintf("%d", resp.StatusCode)},
			cgm.Tag{Category: "source", Value: release.NAME},
		})
		resultLogger.Error().Str("url", c.submissionURL).Str("status", resp.Status).Str("body", string(body)).Msg("submitting telemetry")
		return errors.Errorf("submitting metrics (%s %s)", c.submissionURL, resp.Status)
	}

	c.metrics.IncrementWithTags("collect_submits", cgm.Tags{cgm.Tag{Category: "source", Value: release.NAME}})

	var result TrapResult
	if err := json.Unmarshal(body, &result); err != nil {
		resultLogger.Error().Err(err).Str("body", string(body)).Msg("parsing response")
		return errors.Wrapf(err, "parsing response (%s)", string(body))
	}

	result.CheckUUID = c.checkUUID
	result.SubmitUUID = submitUUID

	resultLogger.Debug().
		Str("duration", time.Since(start).String()).
		Interface("result", result).
		Str("bytes_sent", bytefmt.ByteSize(uint64(dataLen))).
		Msg("submitted")

	if includeStats {
		c.statsmu.Lock()
		c.stats.Metrics += result.Stats
		c.stats.SentBytes += uint64(dataLen)
		c.statsmu.Unlock()
	}

	return nil
}
