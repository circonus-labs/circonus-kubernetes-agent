// Copyright © 2019 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package circonus

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"fmt"
	stdlog "log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"

	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/config"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/config/defaults"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/k8s"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/release"

	apiclient "github.com/circonus-labs/go-apiclient"
	apiclicfg "github.com/circonus-labs/go-apiclient/config"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

const (
	checkStatusActive = "active"
	altCheckType      = "httptrap"
	checkType         = "httptrap:kubernetes"
)

type MetricFilter struct {
	Filter  *regexp.Regexp
	Enabled bool
	Allow   bool
}

type Check struct {
	statsmu              sync.Mutex
	metricsmu            sync.Mutex
	brokerTLSConfig      *tls.Config
	config               *config.Circonus
	client               *http.Client
	metrics              *cgm.CirconusMetrics
	clusterName          string
	checkCID             string
	checkUUID            string
	checkBundleCID       string
	clusterTag           string
	clusterVers          string
	submissionURL        string
	log                  zerolog.Logger
	metricFilters        []MetricFilter
	defaultTags          cgm.Tags
	stats                Stats
	filterDynamicMetrics bool
}

func NewCheck(ctx context.Context, parentLogger zerolog.Logger, cfg *config.Circonus, clusterCfg *config.Cluster) (*Check, error) {
	if cfg == nil {
		return nil, errors.New("invalid circonus config (nil)")
	}
	if clusterCfg == nil {
		return nil, errors.New("invalid cluster config (nil)")
	}
	c := &Check{
		config:      cfg,
		clusterName: clusterCfg.Name,
		clusterTag:  "cluster:" + clusterCfg.Name,
		log:         parentLogger.With().Str("pkg", "circonus.check").Logger(),
	}

	var err error
	c.clusterVers, err = k8s.GetVersion(ctx, clusterCfg)
	if err != nil {
		return nil, err
	}

	// output debug messages for hidden settings which are not DEFAULT
	if cfg.Base64Tags != defaults.Base64Tags {
		c.log.Info().Bool("enabled", cfg.Base64Tags).Msg("base64 tag encoding")
	}
	if cfg.UseGZIP != defaults.UseGZIP {
		c.log.Info().Bool("enabled", cfg.UseGZIP).Msg("gzip submit compression")
	}
	if cfg.DryRun != defaults.DryRun {
		c.log.Info().Bool("enabled", cfg.DryRun).Msg("dry run")
	}
	if cfg.LogAgentMetrics != defaults.LogAgentMetrics {
		c.log.Info().Bool("enabled", cfg.LogAgentMetrics).Msg("log agent metrics")
	}

	if cfg.DefaultStreamtags != "" {
		tagList := strings.Split(cfg.DefaultStreamtags, ",")
		ctags := make(cgm.Tags, len(tagList))
		idx := 0
		for _, t := range tagList {
			td := strings.SplitN(t, ":", 2)
			switch len(td) {
			case 1:
				ctags[idx] = cgm.Tag{Category: td[0]}
			case 2:
				ctags[idx] = cgm.Tag{Category: td[0], Value: td[1]}
			}
			idx++
		}
		c.defaultTags = ctags
	}

	if cfg.DryRun {
		c.log.Info().Msg("dry run enabled, no check required")
		return c, nil // not sending metrics to circonus
	}

	client, err := c.createAPIClient()
	if err != nil {
		return nil, errors.Wrap(err, "setting up circonus api client")
	}

	if err := c.initializeCheckBundle(client); err != nil {
		return nil, err
	}

	initializeAlerting(client, c.log, c.clusterName, c.clusterTag, c.clusterVers, c.checkCID, c.checkUUID)

	{
		cfg := &cgm.Config{
			Log:      stdlog.New(c.log.With().Str("pkg", "cgm").Logger(), "", 0),
			Debug:    c.config.API.Debug,
			Interval: "0",
		}
		cfg.CheckManager.Check.SubmissionURL = c.submissionURL
		m, err := cgm.New(cfg)
		if err != nil {
			c.log.Warn().Err(err).Msg("unable to initialize internal metric submitter")
		}
		c.metrics = m
	}

	return c, nil
}

// UseCompression indicates whether the data being sent should be compressed
func (c *Check) UseCompression() bool {
	return c.config.UseGZIP
}

// LogAgentMetrics will dump the submission request to stdout
func (c *Check) LogAgentMetrics() bool {
	return c.config.LogAgentMetrics
}

// DefaultCGMTags returns the list of default tags in CGM format
func (c *Check) DefaultCGMTags() cgm.Tags {
	return c.defaultTags
}

// createAPIClient initializes and configures a Circonus API client
func (c *Check) createAPIClient() (*apiclient.API, error) {
	c.log.Debug().Msg("initializing api client")
	apiConfig := &apiclient.Config{
		TokenKey: c.config.API.Key,
		TokenApp: c.config.API.App,
		URL:      c.config.API.URL,
		Debug:    c.config.API.Debug,
		Log:      apiLogshim{logh: c.log.With().Str("pkg", "apicli").Logger()},
	}
	if c.config.API.CAFile != "" {
		c.log.Debug().Str("file", c.config.API.CAFile).Msg("adding CA cert to api client")
		cert, err := os.ReadFile(c.config.API.CAFile)
		if err != nil {
			return nil, errors.Wrap(err, "configuring API client")
		}
		cp := x509.NewCertPool()
		if !cp.AppendCertsFromPEM(cert) {
			return nil, errors.New("unable to add API CA Certificate to x509 cert pool")
		}
		apiConfig.TLSConfig = &tls.Config{RootCAs: cp, MinVersion: tls.VersionTLS12}
	}
	client, err := apiclient.New(apiConfig)
	if err != nil {
		return nil, errors.Wrap(err, "creating API client")
	}

	return client, nil
}

// initializeCheckBundle finds or creates a new check bundle
func (c *Check) initializeCheckBundle(client *apiclient.API) error {
	if client == nil {
		return errors.New("invalid state (nil api client)")
	}

	cid := c.config.Check.BundleCID

	if cid != "" {
		bundle, err := client.FetchCheckBundle(apiclient.CIDType(&cid))
		if err != nil {
			return errors.Wrap(err, "fetching configured check bundle")
		}
		if bundle.Status != "active" {
			return errors.Errorf("invalid check bundle (%s), not active", bundle.CID)
		}

		return c.setSubmissionURL(client, bundle)
	}

	bundle, err := c.findOrCreateCheckBundle(client, c.config)
	if err != nil {
		return errors.Wrap(err, "finding/creating check")
	}

	return c.setSubmissionURL(client, bundle)
}

// setSubmissionURL sets the package submissionURL for use by metric submitter
func (c *Check) setSubmissionURL(client *apiclient.API, checkBundle *apiclient.CheckBundle) error {
	bundle, err := c.updateMetricFilters(client, c.config, checkBundle)
	if err != nil {
		return errors.Wrap(err, "updating metric filters")
	}

	c.log.Debug().Interface("check_bundle", bundle).Msg("using check bundle")

	surl, ok := bundle.Config[apiclicfg.SubmissionURL]
	if !ok {
		return errors.Errorf("check bundle config does not have a submission_url (%#v)", bundle.Config)
	}
	c.checkBundleCID = bundle.CID
	if len(bundle.CheckUUIDs) == 1 {
		c.checkUUID = bundle.CheckUUIDs[0]
		c.checkCID = bundle.Checks[0]
	} else {
		c.log.Warn().Int("num_checks", len(bundle.CheckUUIDs)).Msg("multiple check UUIDs found in bundle")
		c.checkUUID = strings.Join(bundle.CheckUUIDs, ",")
		c.checkCID = bundle.Checks[0]
	}
	c.submissionURL = surl
	if err := c.initializeBroker(client, bundle); err != nil {
		return errors.Wrap(err, "unable to initialize broker TLS configuration")
	}

	c.metricFilters = make([]MetricFilter, len(bundle.MetricFilters))
	for idx, filter := range bundle.MetricFilters {
		if len(filter) == 0 {
			return errors.Errorf("invalid (empty) metric filter configured (%d:%v)", idx, filter)
		}

		c.log.Debug().Strs("filter", filter).Msg("adding metric filter")
		c.metricFilters[idx] = MetricFilter{
			Enabled: true,
			Allow:   strings.ToLower(filter[0]) == "allow",
			Filter:  regexp.MustCompile(filter[1]),
		}

		fs := strings.Join(filter, ",")
		// by default all dynamic metrics are sent to the broker
		// we can't filter them locally because the regex is `^.+$`
		// which would catch ALL metrics regardless of source
		// the broker would then have to filter everything
		// increases memory, bandwidth and collection time in larger clusters.
		// default rule:
		// ["allow", "^.+$", "tags", "and(collector:dynamic)", "NO_LOCAL_FILTER dynamically collected metrics"],
		if strings.Contains(fs, "and(collector:dynamic)") {
			if strings.Contains(fs, "NO_LOCAL_FILTER") {
				c.metricFilters[idx].Enabled = false
				c.filterDynamicMetrics = false
			}
		}
	}

	return nil
}

// findOrCreateCheckBundle searches for a check bundle based on target and display name
func (c *Check) findOrCreateCheckBundle(client *apiclient.API, cfg *config.Circonus) (*apiclient.CheckBundle, error) {
	searchCriteria := apiclient.SearchQueryType(fmt.Sprintf(`(active:1)(type:"%s")(host:%s)`, checkType, cfg.Check.Target))

	bundles, err := client.SearchCheckBundles(&searchCriteria, nil)
	if err != nil {
		return nil, errors.Wrapf(err, "searching for check (%s)", searchCriteria)
	}

	if len(*bundles) == 0 {
		c.log.Warn().Str("criteria", string(searchCriteria)).Str("alt_type", altCheckType).Msg("no checks found, searching for alternate check type")
		sc := apiclient.SearchQueryType(fmt.Sprintf(`(active:1)(type:"%s")(host:%s)`, altCheckType, cfg.Check.Target))

		b, err := client.SearchCheckBundles(&sc, nil)
		if err != nil {
			return nil, errors.Wrapf(err, "searching for fallback check (%s)", sc)
		}

		bundles = b
	}

	if len(*bundles) == 0 {
		c.log.Warn().Str("target", cfg.Check.Target).Str("type", checkType).Msg("no active checks found, creating new check")
		return c.createCheckBundle(client, cfg)
	}

	numActive := 0
	checkIdx := -1
	for idx, cb := range *bundles {
		if cb.Status != checkStatusActive {
			continue
		}
		numActive++
		if checkIdx == -1 {
			checkIdx = idx // first match
		}
	}

	if numActive > 1 {
		return nil, errors.Errorf("multiple active checks found (%d) matching (%s)", numActive, searchCriteria)
	}

	bundle := (*bundles)[checkIdx]

	// if the check found was an httptrap instead of httptrap:kubernetes, alert
	if bundle.Type == altCheckType {
		c.log.Warn().Str("alt_type", altCheckType).Str("bundle_cid", bundle.CID).Str("check_uuid", bundle.CheckUUIDs[0]).Msg("found alternate check type, using")
	}

	return &bundle, nil
}

// updateMetricFilters forces check bundle metric filters to match what
// is in deployment configuration which is "source of truth" for filters
func (c *Check) updateMetricFilters(client *apiclient.API, cfg *config.Circonus, b *apiclient.CheckBundle) (*apiclient.CheckBundle, error) {
	checkMetricFilters := c.loadMetricFilters()
	if cfg.Check.MetricFilters != "" {
		var filters [][]string
		if e := json.Unmarshal([]byte(cfg.Check.MetricFilters), &filters); e != nil {
			return nil, errors.Wrap(e, "parsing check bundle metric filters")
		}
		checkMetricFilters = filters
	}

	if !strings.Contains(strings.Join(b.Tags, ","), c.clusterTag) {
		b.Tags = append(b.Tags, c.clusterTag)
	}

	b.MetricFilters = checkMetricFilters
	bundle, err := client.UpdateCheckBundle(b)
	if err != nil {
		return nil, errors.Wrap(err, "updating check bundle metric filters")
	}

	return bundle, nil
}

// createCheckBundle creates a new check bundle
func (c *Check) createCheckBundle(client *apiclient.API, cfg *config.Circonus) (*apiclient.CheckBundle, error) {

	secret, err := makeSecret()
	if err != nil {
		secret = "myS3cr3t"
	}

	notes := fmt.Sprintf("%s-%s", release.NAME, release.VERSION)

	checkMetricFilters := c.loadMetricFilters()
	if cfg.Check.MetricFilters != "" {
		var filters [][]string
		if e := json.Unmarshal([]byte(cfg.Check.MetricFilters), &filters); e != nil {
			return nil, errors.Wrap(e, "parsing check bundle metric filters")
		}
		checkMetricFilters = filters
	}

	tagList := strings.Split(cfg.Check.Tags, ",")
	tagList = append(tagList, c.clusterTag)

	checkConfig := &apiclient.CheckBundle{
		Brokers: []string{cfg.Check.BrokerCID},
		Config: apiclient.CheckBundleConfig{
			"asynch_metrics": "true",
			"secret":         secret,
		},
		DisplayName:   cfg.Check.Title,
		MetricFilters: checkMetricFilters,
		MetricLimit:   apiclicfg.DefaultCheckBundleMetricLimit,
		Metrics:       []apiclient.CheckBundleMetric{},
		Notes:         &notes,
		Period:        60,
		Status:        checkStatusActive,
		Tags:          tagList,
		Target:        cfg.Check.Target,
		Timeout:       10,
		Type:          checkType,
	}

	bundle, err := client.CreateCheckBundle(checkConfig)
	if err != nil {
		return nil, errors.Wrap(err, "creating check")
	}

	return bundle, nil
}

func makeSecret() (string, error) {
	hash := sha256.New()
	x := make([]byte, 2048)
	if _, err := rand.Read(x); err != nil {
		return "", err
	}
	if _, err := hash.Write(x); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil))[0:16], nil
}
