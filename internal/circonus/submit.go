// Copyright © 2019 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package circonus

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"path"
	"strconv"
	"time"

	"code.cloudfoundry.org/bytefmt"
	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/release"
	"github.com/google/uuid"
	"github.com/hashicorp/go-retryablehttp"
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

func (c *Check) FlushCGM(ctx context.Context) {
	if c.metrics != nil {
		m := c.metrics.FlushMetrics()
		// c.log.Info().Interface("metrics", m).Msg("sending metrics")
		data, err := json.Marshal(m)
		if err != nil {
			c.log.Warn().Err(err).Msg("encoding metrics")
			return
		}
		if err := c.SubmitStream(ctx, bytes.NewReader(data), c.log); err != nil {
			c.log.Error().Err(err).Msg("submitting cgm metrics")
		}
	}
}

func (c *Check) ResetSubmitStats() {
	c.statsmu.Lock()
	defer c.statsmu.Unlock()
	c.stats.Metrics = 0
	c.stats.SentBytes = 0
}

func (c *Check) SubmitStats() Stats {
	c.statsmu.Lock()
	defer c.statsmu.Unlock()
	return Stats{
		Metrics:   c.stats.Metrics,
		SentBytes: c.stats.SentBytes,
		SentSize:  bytefmt.ByteSize(c.stats.SentBytes),
	}
}

func (c *Check) SubmitQueue(ctx context.Context, metrics map[string]MetricSample, resultLogger zerolog.Logger) error {
	if metrics == nil {
		return errors.New("invalid metrics (nil)")
	}

	data, err := json.MarshalIndent(metrics, "", "  ")
	if err != nil {
		return errors.Wrap(err, "marshaling metrics")
	}

	return c.SubmitStream(ctx, bytes.NewReader(data), resultLogger)
}

// SubmitStream sends metrics to a circonus trap
func (c *Check) SubmitStream(ctx context.Context, metrics io.Reader, resultLogger zerolog.Logger) error {
	if metrics == nil {
		return errors.New("invalid metrics (nil)")
	}

	start := time.Now()

	if c.submissionURL == "" {
		if c.config.DryRun {
			_, err := io.Copy(os.Stdout, metrics)
			return err
		}
		return errors.New("no submission url and not in dry-run mode")
	}

	var client *http.Client

	if c.brokerTLSConfig != nil {
		client = &http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
				DialContext: (&net.Dialer{
					Timeout:   10 * time.Second,
					KeepAlive: 3 * time.Second,
					DualStack: true,
				}).DialContext,
				TLSClientConfig:     c.brokerTLSConfig,
				TLSHandshakeTimeout: 10 * time.Second,
				DisableKeepAlives:   false,
				MaxIdleConnsPerHost: 2,
				DisableCompression:  false,
			},
		}
	} else {
		client = &http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
				DialContext: (&net.Dialer{
					Timeout:   10 * time.Second,
					KeepAlive: 3 * time.Second,
					DualStack: true,
				}).DialContext,
				DisableKeepAlives:   false,
				MaxIdleConnsPerHost: 2,
				DisableCompression:  false,
			},
		}
	}

	submitUUID, err := uuid.NewRandom()
	if err != nil {
		resultLogger.Error().Err(err).Msg("creating new submit ID")
		return errors.Wrap(err, "creating new submit ID")
	}

	rawData, err := ioutil.ReadAll(metrics)
	if err != nil {
		resultLogger.Error().Err(err).Msg("reading metric data")
		return errors.Wrap(err, "reading metric data")
	}

	payloadIsCompressed := false

	var subData *bytes.Buffer
	if c.UseCompression() && len(rawData) > compressionThreshold {
		subData = bytes.NewBuffer([]byte{})
		zw := gzip.NewWriter(subData)
		n, err := zw.Write(rawData)
		if err != nil {
			resultLogger.Error().Err(err).Msg("compressing metrics")
			return errors.Wrap(err, "compressing metrics")
		}
		if n != len(rawData) {
			resultLogger.Error().Err(err).Int("data_len", len(rawData)).Int("written", n).Msg("gzip write length mismatch")
			return errors.Errorf("write length mismatch data length %d != written length %d", len(rawData), n)
		}
		if err := zw.Close(); err != nil {
			resultLogger.Error().Err(err).Msg("closing gzip writer")
			return errors.Wrap(err, "closing gzip writer")
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
		fh, err := os.Create(fn)
		if err != nil {
			c.log.Error().Err(err).Str("file", fn).Msg("skipping submit trace")
		} else {
			if _, err := fh.Write(subData.Bytes()); err != nil {
				resultLogger.Error().Err(err).Msg("writing metric trace")
			}
			if err := fh.Close(); err != nil {
				resultLogger.Error().Err(err).Str("file", fn).Msg("closing metric trace")
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

	if c.DebugSubmissions() {
		dump, err := httputil.DumpRequestOut(req.Request, !payloadIsCompressed)
		if err != nil {
			resultLogger.Error().Err(err).Msg("dumping request")
			return err
		}

		fmt.Println(string(dump))
	}

	retryClient := retryablehttp.NewClient()
	retryClient.HTTPClient = client
	retryClient.Logger = logshim{logh: c.log.With().Str("pkg", "retryablehttp").Logger()}
	retryClient.RetryWaitMin = 50 * time.Millisecond
	retryClient.RetryWaitMax = 2 * time.Second
	retryClient.RetryMax = 10
	retryClient.RequestLogHook = func(l retryablehttp.Logger, r *http.Request, attempt int) {
		if attempt > 0 {
			c.metrics.IncrementWithTags("collect_submit_retries", cgm.Tags{cgm.Tag{Category: "source", Value: release.NAME}})
			reqStart = time.Now()
			resultLogger.Warn().Str("url", r.URL.String()).Int("attempt", attempt).Msg("retrying...")
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
	if err != nil {
		resultLogger.Error().Err(err).Msg("making request")
		return err
	}

	body, err := ioutil.ReadAll(resp.Body)
	_ = resp.Body.Close()
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

	c.statsmu.Lock()
	c.stats.Metrics += result.Stats
	c.stats.SentBytes += uint64(dataLen)
	c.statsmu.Unlock()

	return nil
}
