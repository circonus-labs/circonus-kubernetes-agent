// Copyright © 2019 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// Package defaults contains the default values for configuration options
package defaults

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/circonus-labs/circonus-kubernetes-agent/internal/release"
)

const (
	// Circonus defaults

	APITokenKey        = ""
	APITokenKeyFile    = ""
	APITokenApp        = release.NAME
	APIURL             = "https://api.circonus.com/v2/"
	APIDebug           = false
	APICAFile          = ""
	CheckBundleCID     = ""
	CheckCreate        = true
	CheckBrokerCID     = "/broker/35" // circonus public httptrap broker
	CheckBrokerCAFile  = ""
	CheckMetricFilters = ""
	CheckTags          = ""
	CheckTarget        = "" // defaults to cluster name
	DefaultStreamtags  = ""
	MetricFiltersFile  = "/ck8sa/metric-filters.json" // assumes running in a pod, ConfigMap mounted volume
	DefaultAlertsFile  = "/ck8sa/default-alerts.json" // assumes running in a pod, ConfigMap mounted volume
	CustomRulesFile    = "/ck8sa/custom-rules.json"   // assumes running in a pod, ConfigMap mounted volume
	CheckTitle         = ""
	TraceSubmits       = ""
	// hidden circonus settings for development and debugging
	DryRun = false
	// StreamMetrics = false
	// these hidden settings are mainly for debugging
	// the features default to ON and can be toggled OFF
	NoBase64        = false
	Base64Tags      = true
	NoGZIP          = false
	UseGZIP         = true
	LogAgentMetrics = false
	NodeCC          = false

	// General defaults

	Debug     = false
	LogLevel  = "info"
	LogPretty = false

	// Kubernetes cluster

	/*
		Assets in the pod when running in a deployment within the cluster:

		BearerTokenFile: /var/run/secrets/kubernetes.io/serviceaccount/token
		URL: https://kubernetes
		CAFile: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
		namespace of ck8sa: /var/run/secrets/kubernetes.io/serviceaccount/namespace
	*/

	K8SName                   = ""
	K8SInterval               = "1m"
	K8SAPIURL                 = "https://kubernetes.default.svc"                       // https://kubernetes.io/docs/tasks/access-application-cluster/access-cluster/#accessing-the-api-from-a-pod
	K8SAPICAFile              = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt" // https://kubernetes.io/docs/tasks/access-application-cluster/access-cluster/#accessing-the-api-from-a-pod
	K8SBearerToken            = ""
	K8SBearerTokenFile        = "/var/run/secrets/kubernetes.io/serviceaccount/token" //nolint:gosec // https://kubernetes.io/docs/tasks/access-application-cluster/access-cluster/#accessing-the-api-from-a-pod
	K8SEnableEvents           = true                                                  // dashboard
	K8SEnableKubeStateMetrics = true                                                  // dashobard
	K8SKSMFieldSelectorQuery  = "metadata.name=kube-state-metrics"                    // default from 'standard' service deployment, https://github.com/kubernetes/kube-state-metrics/blob/master/examples/standard/service.yaml#L19
	K8SKSMMetricsPort         = ""                                                    // no default, pulled from endpoint for service
	K8SKSMMetricsPortName     = "http-metrics"                                        // default from 'standard' service deployment, https://github.com/kubernetes/kube-state-metrics/blob/master/examples/standard/service.yaml#L11
	K8SKSMRequestMode         = "direct"                                              // DEPRECATED - 'direct' or 'proxy' modes supported
	K8SKSMTelemetryPortName   = "telemetry"                                           // DEPRECATED - default from 'standard' service deployment, https://github.com/kubernetes/kube-state-metrics/blob/master/examples/standard/service.yaml#L11
	K8SEnableAPIServer        = true                                                  // dashboard
	K8SEnableMetricsServer    = false                                                 // deprecated
	K8SEnableNodes            = true                                                  // dashboard
	K8SEnableNodeStats        = true                                                  // dashboard
	K8SEnableNodeMetrics      = true                                                  // dashboard
	K8SEnableCadvisorMetrics  = false                                                 // not needed by dashboard and is deprecated by k8s
	K8SEnableKubeDNSMetrics   = true                                                  // dashboard
	K8SKubeDNSMetricsPort     = "10054"                                               // ONLY used if the kube-dns service does not have scrape and port annotations (e.g. GKE)
	K8SNodeSelector           = ""                                                    // blank=all
	K8SIncludePods            = true                                                  // dashboard
	K8SPodLabelKey            = ""                                                    // blank=all
	K8SPodLabelVal            = ""                                                    // blank=all
	K8SIncludeContainers      = false                                                 // not needed by dashboard
	K8SAPITimelimit           = "10s"                                                 // default timeout
	K8SDynamicCollectorFile   = "/ck8sa/dynamic-collectors.yaml"                      // assumes running in a pod, ConfigMap mounted volume
)

var (
	// BasePath is the "base" directory
	//
	// expected installation structure:
	// base        (e.g. /opt/circonus/k8s-agent)
	//   /etc      (e.g. /opt/circonus/k8s-agent/etc)
	//   /sbin     (e.g. /opt/circonus/k8s-agent/sbin)
	BasePath = ""

	// EtcPath returns the default etc directory within base directory
	EtcPath = ""

	// ConfigFile defines the default configuration file name
	ConfigFile = ""

	// K8SNodePoolSize defaults to number of available cpus for concurrent collection of metrics
	K8SNodePoolSize = runtime.NumCPU()
)

func init() {
	var exePath string
	var resolvedExePath string
	var err error

	exePath, err = os.Executable()
	if err == nil {
		resolvedExePath, err = filepath.EvalSymlinks(exePath)
		if err == nil {
			BasePath = filepath.Clean(filepath.Join(filepath.Dir(resolvedExePath), ".."))
		}
	}

	if err != nil {
		fmt.Printf("Unable to determine path to binary %v\n", err)
		os.Exit(1)
	}

	EtcPath = filepath.Join(BasePath, "etc")
	ConfigFile = filepath.Join(EtcPath, release.NAME+".yaml")
}
