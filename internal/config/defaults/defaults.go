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
	CheckTitle         = ""
	TraceSubmits       = ""
	// hidden circonus settings for development and debugging
	DryRun        = false
	StreamMetrics = false
	// these hidden settings are mainly for debugging
	// the features default to ON and can be toggled OFF
	NoBase64   = false
	Base64Tags = true
	NoGZIP     = false
	UseGZIP    = true

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
	K8SAPIURL                 = "https://kubernetes"
	K8SAPICAFile              = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
	K8SBearerToken            = ""
	K8SBearerTokenFile        = "/var/run/secrets/kubernetes.io/serviceaccount/token" //nolint:gosec
	K8SEnableEvents           = false
	K8SEnableKubeStateMetrics = false
	K8SEnableMetricsServer    = false
	K8SEnableNodes            = true
	K8SNodeSelector           = "" // blank=all
	K8SIncludePods            = true
	K8SPodLabelKey            = "" // blank=all
	K8SPodLabelVal            = "" // blank=all
	K8SIncludeContainers      = false
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

	// CheckTarget defaults to return from os.Hostname()
	CheckTarget = ""

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

	CheckTarget, err = os.Hostname()
	if err != nil {
		fmt.Printf("Unable to determine hostname for target %v\n", err)
		os.Exit(1)
	}
}
