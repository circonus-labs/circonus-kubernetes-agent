// Copyright © 2019 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package circonus

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	stdlog "log"
	"net/http"
	"os"
	"path"
	"regexp"
	"strings"
	"sync"

	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/config"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/config/defaults"
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

type Stats struct {
	Metrics   uint64
	SentBytes uint64
	SentSize  string
}

type MetricSet struct {
	Metrics []byte
	Logger  zerolog.Logger
}

type MetricFilter struct {
	Allow  bool
	Filter *regexp.Regexp
}
type Check struct {
	config          *config.Circonus
	brokerTLSConfig *tls.Config
	checkBundleCID  string
	checkUUID       string
	submissionURL   string
	log             zerolog.Logger
	stats           Stats
	statsmu         sync.Mutex
	metrics         *cgm.CirconusMetrics
	defaultTags     cgm.Tags
	metricQueue     chan MetricSet
	mQueue          chan Metric
	metricFilters   []MetricFilter
	client          *http.Client
}

func NewCheck(parentLogger zerolog.Logger, cfg *config.Circonus) (*Check, error) {
	if cfg == nil {
		return nil, errors.New("invalid circonus config (nil)")
	}
	c := &Check{
		config:      cfg,
		log:         parentLogger.With().Str("pkg", "circonus.check").Logger(),
		metricQueue: make(chan MetricSet, 5),
		mQueue:      make(chan Metric, 1000),
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
	if cfg.DebugSubmissions != defaults.DebugSubmissions {
		c.log.Info().Bool("enabled", cfg.DebugSubmissions).Msg("debug submissions")
	}
	if cfg.SerialSubmissions != defaults.SerialSubmissions {
		c.log.Info().Bool("enabled", cfg.SerialSubmissions).Msg("serial submissions")
	}
	if cfg.MaxMetricBucketSize != defaults.MaxMetricBucketSize {
		c.log.Info().Int("max_metric_bucket_size", cfg.MaxMetricBucketSize).Msg("max metric bucket size")
	}

	if cfg.DefaultStreamtags != "" {
		ctags := cgm.Tags{}
		tagList := strings.Split(cfg.DefaultStreamtags, ",")
		for _, t := range tagList {
			td := strings.SplitN(t, ":", 2)
			if len(td) == 2 {
				ctags = append(ctags, cgm.Tag{Category: td[0], Value: td[1]})
			}
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

// MaxMetricBucketSize used by promtext parser to bucket metrics for submissions (may stabilize memory with large prom output)
func (c *Check) MaxMetricBucketSize() int {
	return c.config.MaxMetricBucketSize
}

// ConcurrentSubmissions enable sending metrics to circonus concurrently
// when disabled collection time is increased, when enabled may produce gaps
func (c *Check) ConcurrentSubmissions() bool {
	return c.config.ConcurrentSubmissions
}

// UseCompression indicates whether the data being sent should be compressed
func (c *Check) UseCompression() bool {
	return c.config.UseGZIP
}

// DebugSubmissions will dump the submission request to stdout
func (c *Check) DebugSubmissions() bool {
	return c.config.DebugSubmissions
}

// createAPIClient initializes and configures a Circonus API client
func (c *Check) createAPIClient() (*apiclient.API, error) {
	c.log.Debug().Msg("initializing api client")
	apiConfig := &apiclient.Config{
		TokenKey: c.config.API.Key,
		TokenApp: c.config.API.App,
		URL:      c.config.API.URL,
		Debug:    c.config.API.Debug,
		Log:      logshim{logh: c.log.With().Str("pkg", "apicli").Logger()},
	}
	if c.config.API.CAFile != "" {
		cert, err := ioutil.ReadFile(c.config.API.CAFile)
		if err != nil {
			return nil, errors.Wrap(err, "configuring API client")
		}
		cp := x509.NewCertPool()
		if !cp.AppendCertsFromPEM(cert) {
			return nil, errors.New("unable to add API CA Certificate to x509 cert pool")
		}
		apiConfig.TLSConfig = &tls.Config{RootCAs: cp}
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
	} else {
		c.log.Warn().Int("num_checks", len(bundle.CheckUUIDs)).Msg("multiple check UUIDs found in bundle")
		c.checkUUID = strings.Join(bundle.CheckUUIDs, ",")
	}
	c.submissionURL = surl
	if err := c.initializeBroker(client, bundle); err != nil {
		return errors.Wrap(err, "unable to initialize broker TLS configuration")
	}

	c.metricFilters = make([]MetricFilter, len(bundle.MetricFilters))
	for idx, filter := range bundle.MetricFilters {
		if len(filter) == 0 {
			return errors.Errorf("invalid metric filters configured (%d:%v)", idx, filter)
		}

		c.log.Debug().Strs("filter", filter).Msg("adding metric filter")
		c.metricFilters[idx] = MetricFilter{
			Allow:  filter[0] == "allow",
			Filter: regexp.MustCompile(filter[1]),
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
		Tags:          strings.Split(cfg.Check.Tags, ","),
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

type metricFilters struct {
	Filters [][]string `json:"metric_filters"`
}

func (c *Check) loadMetricFilters() [][]string {
	defaults := [][]string{
		{"allow", "^[rt]x$", "tags", "and(resource:network,or(units:bytes,units:errors),not(container_name:*),not(sys_container:*))", "utilization"},
		{"allow", "^(used|capacity)$", "tags", "and(or(units:bytes,units:percent),or(resource:memory,resource:fs,volume_name:*),not(container_name:*),not(sys_container:*))", "utilization"},
		{"allow", "^usageNanoCores$", "tags", "and(not(container_name:*),not(sys_container:*))", "utilization"},
		{"allow", "^apiserver_request_total$", "tags", "and(or(code:5*,code:4*))", "api req errors"},
		{"allow", "^authenticated_user_requests$", "api auth"},
		{"allow", "^authentication_attempts$", "api auth"},
		{"allow", "^kube_pod_container_status_(running|terminated|waiting|ready)$", "containers"},
		{"allow", "^kube_deployment_(created|spec_replicas|status_replicas|status_replicas_updated|status_replicas_available|status_replicas_unavailable)$", "deployments"},
		{"allow", "^kube_pod_start_time", "pods"},
		{"allow", "^kube_pod_status_phase$", "tags", "and(or(phase:Running,phase:Pending,phase:Failed,phase:Succeeded))", "pods"},
		{"allow", "^kube_pod_status_(ready|scheduled)$", "tags", "and(condition:true)", "pods"},
		{"allow", "^kube_(service_labels|deployment_labels|pod_container_info|pod_deleted)$", "ksm inventory"},
		{"allow", "^(node|kubelet_running_pod_count|Ready)$", "nodes"},
		{"allow", "^NetworkUnavailable$", "node status"},
		{"allow", "^(Disk|Memory|PID)Pressure$", "node status"},
		{"allow", "^capacity_.*$", "node capacity"},
		{"allow", "^kube_namespace_status_phase$", "tags", "and(or(phase:Active,phase:Terminating))", "namespaces"},
		{"allow", "^coredns_.*$", "kube-dns"},
		{"allow", "^events$", "events"},
		{"allow", "^collect_.*$", "agent collection stats"},
		{"deny", "^.+$", "all other metrics}"},
	}

	mfConfigFile := path.Join(string(os.PathSeparator), "ck8sa", "metric-filters.json")
	data, err := ioutil.ReadFile(mfConfigFile)
	if err != nil {
		c.log.Warn().Err(err).Str("metric_filter_config", mfConfigFile).Msg("using defaults")
		return defaults
	}

	var mf metricFilters
	if err := json.Unmarshal(data, &mf); err != nil {
		c.log.Warn().Err(err).Str("metric_filter_config", mfConfigFile).Msg("using defaults")
		return defaults
	}

	return mf.Filters
}
