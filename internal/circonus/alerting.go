// Copyright © 2020 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package circonus

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/circonus-labs/circonus-kubernetes-agent/internal/config/keys"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/release"
	apiclient "github.com/circonus-labs/go-apiclient"
	"github.com/hashicorp/go-version"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"
)

const defaultRuleSetsStr119 = `
{
    "crashloops_container": {
        "filter": "and(reason:CrashLoopBackOff)",
        "lookup_key": "k8s_health_crashloop_1",
        "metric_name": "kube_pod_container_status_waiting_reason",
        "metric_type": "numeric",
        "name": "Kubernetes CrashLoops ({cluster_name})",
        "rules": [
			{
				"wait": 0,
				"severity": 0,
				"windowing_duration": 300,
				"windowing_min_duration": 0,
				"criteria": "on absence",
				"windowing_function": null,
				"value": 900
        	},
            {
                "criteria": "max value",
                "severity": 1,
                "wait": 0,
                "windowing_duration": 300,
                "value": "0"
            }
        ]
    },
    "crashloops_init_container": {
        "filter": "and(reason:CrashLoopBackOff)",
        "lookup_key": "k8s_health_crashloop_2",
        "metric_name": "kube_pod_init_container_status_waiting_reason",
        "metric_type": "numeric",
        "name": "Kubernetes CrashLoops (Init) ({cluster_name})",
        "rules": [
			{
				"wait": 0,
				"severity": 0,
				"windowing_duration": 300,
				"windowing_min_duration": 0,
				"criteria": "on absence",
				"windowing_function": null,
				"value": 900
        	},
            {
                "criteria": "max value",
                "severity": 1,
                "wait": 0,
                "windowing_duration": 300,
                "value": "0"
            }
        ]
    },
    "cpu_utilization": {
        "filter": "and(resource:cpu)",
        "lookup_key": "k8s_health_cpu",
        "metric_name": "utilization",
        "metric_type": "numeric",
        "name": "Kubernetes CPU ({cluster_name})",
        "rules": [
			{
				"wait": 0,
				"severity": 0,
				"windowing_duration": 300,
				"windowing_min_duration": 0,
				"criteria": "on absence",
				"windowing_function": null,
				"value": 900
        	},
            {
                "criteria": "max value",
                "severity": 1,
                "wait": 0,
                "windowing_duration": 900,
				"windowing_function": "average",
				"windowing_min_duration": 900,
                "value": "75"
            }
        ]    
    },
    "disk_pressure": {
        "filter": "and(condition:DiskPressure,status:true)",
        "lookup_key": "k8s_health_disk",
        "metric_name": "kube_node_status_condition",
        "metric_type": "numeric",
        "name": "Kubernetes Disk Pressure ({cluster_name})",
        "rules": [
			{
				"wait": 0,
				"severity": 0,
				"windowing_duration": 300,
				"windowing_min_duration": 0,
				"criteria": "on absence",
				"windowing_function": null,
				"value": 900
        	},
            {
                "criteria": "max value",
                "severity": 1,
                "wait": 0,
                "windowing_duration": 300,
                "value": "0"
            }
        ]
    },
    "memory_pressure": {
        "filter": "and(condition:MemoryPressure,status:true)",
        "lookup_key": "k8s_health_memory",
        "metric_name": "kube_node_status_condition",
        "metric_type": "numeric",
        "name": "Kubernetes Memory Pressure ({cluster_name})",
        "rules": [
			{
				"wait": 0,
				"severity": 0,
				"windowing_duration": 300,
				"windowing_min_duration": 0,
				"criteria": "on absence",
				"windowing_function": null,
				"value": 900
        	},
            {
                "criteria": "max value",
                "severity": 1,
                "wait": 0,
                "windowing_duration": 300,
                "value": "0"
            }
        ]
    },
    "pid_pressure": {
        "filter": "and(condition:PIDPressure,status:true)",
        "lookup_key": "k8s_health_pid",
        "metric_name": "kube_node_status_condition",
        "metric_type": "numeric",
        "name": "Kubernetes PID Pressure ({cluster_name})",
        "rules": [
			{
				"wait": 0,
				"severity": 0,
				"windowing_duration": 300,
				"windowing_min_duration": 0,
				"criteria": "on absence",
				"windowing_function": null,
				"value": 900
        	},
            {
                "criteria": "max value",
                "severity": 1,
                "wait": 0,
                "windowing_duration": 300,
                "value": "0"
            }
        ]
    },
    "network_unavailable": {
        "filter": "and(condition:NetworkUnavailable,status:true)",
        "lookup_key": "k8s_health_network",
        "metric_name": "kube_node_status_condition",
        "metric_type": "numeric",
        "name": "Kubernetes Network Unavailable ({cluster_name})",
        "rules": [
			{
				"wait": 0,
				"severity": 0,
				"windowing_duration": 300,
				"windowing_min_duration": 0,
				"criteria": "on absence",
				"windowing_function": null,
				"value": 900
        	},
            {
                "criteria": "max value",
                "severity": 1,
				"wait": 0,
				"windowing_duration": 300,
				"windowing_function": "average",
				"windowing_min_duration": 300,
                "value": "0.99"
            }
        ]
    },
    "job_failures": {
        "filter": "and(job_name:*)",
        "lookup_key": "k8s_health_jobs",
        "metric_name": "kube_job_status_failed",
        "metric_type": "numeric",
        "name": "Kubernetes Job Failures ({cluster_name})",
        "rules": [
			{
				"wait": 0,
				"severity": 0,
				"windowing_duration": 300,
				"windowing_min_duration": 0,
				"criteria": "on absence",
				"windowing_function": null,
				"value": 900
        	},
            {
                "criteria": "max value",
                "severity": 1,
                "wait": 0,
                "windowing_duration": 300,
                "value": "0"
            }
        ]        
    },
    "persistent_volume_failures": {
        "filter": "and(phase:Failed)",
        "lookup_key": "k8s_health_pvols",
        "metric_name": "kube_persistentvolume_status_phase",
        "metric_type": "numeric",
        "name": "Kubernetes Persistent Volume Failures ({cluster_name})",
        "rules": [
			{
				"wait": 0,
				"severity": 0,
				"windowing_duration": 300,
				"windowing_min_duration": 0,
				"criteria": "on absence",
				"windowing_function": null,
				"value": 900
        	},
            {
                "criteria": "max value",
                "severity": 1,
                "wait": 0,
                "windowing_duration": 300,
                "value": "0"
            }
        ]        
    },
    "pod_pending_delays": {
        "filter": "and(phase:Pending)",
        "lookup_key": "k8s_health_poddelays",
        "metric_name": "kube_pod_status_phase",
        "metric_type": "numeric",
        "name": "Kubernetes Pod Pending Delays ({cluster_name})",
        "rules": [
			{
				"wait": 0,
				"severity": 0,
				"windowing_duration": 300,
				"windowing_min_duration": 0,
				"criteria": "on absence",
				"windowing_function": null,
				"value": 900
        	},
            {
                "criteria": "max value",
                "severity": 1,
                "wait": 0,
                "windowing_duration": 900,
				"windowing_function": "average",
				"windowing_min_duration": 900,
                "value": "0.99"
            }
        ]        
    },
    "deployment_glitches": {
        "filter": "and(deployment:*)",
        "lookup_key": "k8s_health_deploys",
        "metric_name": "deployment_generation_delta",
        "metric_type": "numeric",
        "name": "Kubernetes Deployment Glitches ({cluster_name})",
        "rules": [
			{
				"wait": 0,
				"severity": 0,
				"windowing_duration": 300,
				"windowing_min_duration": 0,
				"criteria": "on absence",
				"windowing_function": null,
				"value": 900
        	},
            {
                "criteria": "max value",
                "severity": 1,
                "wait": 0,
                "windowing_duration": 300,
                "value": "0"
            },
            {
                "criteria": "min value",
                "severity": 1,
                "wait": 0,
                "windowing_duration": 300,
                "value": "0"
            }
        ]        
    },
    "daemonsets_not_ready": {
        "filter": "and(daemonset:*)",
        "lookup_key": "k8s_health_daemonsets",
        "metric_name": "daemonset_scheduled_delta",
        "metric_type": "numeric",
        "name": "Kubernetes DaemonSets Not Ready ({cluster_name})",
        "rules": [
			{
				"wait": 0,
				"severity": 0,
				"windowing_duration": 300,
				"windowing_min_duration": 0,
				"criteria": "on absence",
				"windowing_function": null,
				"value": 900
        	},
            {
                "criteria": "max value",
                "severity": 1,
                "wait": 0,
                "windowing_duration": 300,
                "value": "0"
            },
            {
                "criteria": "min value",
                "severity": 1,
                "wait": 0,
                "windowing_duration": 300,
                "value": "0"
            }
        ]
    },
    "statefulsets_not_ready": {
        "filter": "and(statefulset:*)",
        "lookup_key": "k8s_health_statefulsets",
        "metric_name": "statefulset_replica_delta",
        "metric_type": "numeric",
        "name": "Kubernetes StatefulSets Not Ready ({cluster_name})",
        "rules": [
			{
				"wait": 0,
				"severity": 0,
				"windowing_duration": 300,
				"windowing_min_duration": 0,
				"criteria": "on absence",
				"windowing_function": null,
				"value": 900
        	},
            {
                "criteria": "max value",
                "severity": 1,
                "wait": 0,
                "windowing_duration": 300,
                "value": "0"
            },
            {
                "criteria": "min value",
                "severity": 1,
                "wait": 0,
                "windowing_duration": 300,
                "value": "0"
            }
        ]
    }
}
`

const defaultRuleSetsStr120 = `
{
    "crashloops_container": {
        "filter": "and(reason:CrashLoopBackOff)",
        "lookup_key": "k8s_health_crashloop_1",
        "metric_name": "kube_pod_container_status_waiting_reason",
        "metric_type": "numeric",
        "name": "Kubernetes CrashLoops ({cluster_name})",
        "rules": [
			{
				"wait": 0,
				"severity": 0,
				"windowing_duration": 300,
				"windowing_min_duration": 0,
				"criteria": "on absence",
				"windowing_function": null,
				"value": 900
        	},
            {
                "criteria": "max value",
                "severity": 1,
                "wait": 0,
                "windowing_duration": 300,
                "value": "0"
            }
        ]
    },
    "crashloops_init_container": {
        "filter": "and(reason:CrashLoopBackOff)",
        "lookup_key": "k8s_health_crashloop_2",
        "metric_name": "kube_pod_init_container_status_waiting_reason",
        "metric_type": "numeric",
        "name": "Kubernetes CrashLoops (Init) ({cluster_name})",
        "rules": [
			{
				"wait": 0,
				"severity": 0,
				"windowing_duration": 300,
				"windowing_min_duration": 0,
				"criteria": "on absence",
				"windowing_function": null,
				"value": 900
        	},
            {
                "criteria": "max value",
                "severity": 1,
                "wait": 0,
                "windowing_duration": 300,
                "value": "0"
            }
        ]
    },
    "cpu_utilization": {
        "filter": "",
        "lookup_key": "k8s_health_cpu",
        "metric_name": "node_cpu_usage_seconds_total",
        "metric_type": "numeric",
        "name": "Kubernetes CPU ({cluster_name})",
        "rules": [
		{
			"wait": 0,
			"severity": 0,
			"windowing_duration": 300,
			"windowing_min_duration": 0,
			"criteria": "on absence",
			"windowing_function": null,
			"value": 900
        	},
		{
                	"criteria": "max value",
                	"severity": 1,
                	"wait": 0,
                	"windowing_duration": 900,
                	"windowing_function": "average",
                	"windowing_min_duration": 900,
                	"value": "0.75"
		}
        ],
    },
    "disk_pressure": {
        "filter": "and(condition:DiskPressure,status:true)",
        "lookup_key": "k8s_health_disk",
        "metric_name": "kube_node_status_condition",
        "metric_type": "numeric",
        "name": "Kubernetes Disk Pressure ({cluster_name})",
        "rules": [
			{
				"wait": 0,
				"severity": 0,
				"windowing_duration": 300,
				"windowing_min_duration": 0,
				"criteria": "on absence",
				"windowing_function": null,
				"value": 900
        	},
            {
                "criteria": "max value",
                "severity": 1,
                "wait": 0,
                "windowing_duration": 300,
                "value": "0"
            }
        ]
    },
    "memory_pressure": {
        "filter": "and(condition:MemoryPressure,status:true)",
        "lookup_key": "k8s_health_memory",
        "metric_name": "kube_node_status_condition",
        "metric_type": "numeric",
        "name": "Kubernetes Memory Pressure ({cluster_name})",
        "rules": [
			{
				"wait": 0,
				"severity": 0,
				"windowing_duration": 300,
				"windowing_min_duration": 0,
				"criteria": "on absence",
				"windowing_function": null,
				"value": 900
        	},
            {
                "criteria": "max value",
                "severity": 1,
                "wait": 0,
                "windowing_duration": 300,
                "value": "0"
            }
        ]
    },
    "pid_pressure": {
        "filter": "and(condition:PIDPressure,status:true)",
        "lookup_key": "k8s_health_pid",
        "metric_name": "kube_node_status_condition",
        "metric_type": "numeric",
        "name": "Kubernetes PID Pressure ({cluster_name})",
        "rules": [
			{
				"wait": 0,
				"severity": 0,
				"windowing_duration": 300,
				"windowing_min_duration": 0,
				"criteria": "on absence",
				"windowing_function": null,
				"value": 900
        	},
            {
                "criteria": "max value",
                "severity": 1,
                "wait": 0,
                "windowing_duration": 300,
                "value": "0"
            }
        ]
    },
    "network_unavailable": {
        "filter": "and(condition:NetworkUnavailable,status:true)",
        "lookup_key": "k8s_health_network",
        "metric_name": "kube_node_status_condition",
        "metric_type": "numeric",
        "name": "Kubernetes Network Unavailable ({cluster_name})",
        "rules": [
			{
				"wait": 0,
				"severity": 0,
				"windowing_duration": 300,
				"windowing_min_duration": 0,
				"criteria": "on absence",
				"windowing_function": null,
				"value": 900
        	},
            {
                "criteria": "max value",
                "severity": 1,
				"wait": 0,
				"windowing_duration": 300,
				"windowing_function": "average",
				"windowing_min_duration": 300,
                "value": "0.99"
            }
        ]
    },
    "job_failures": {
        "filter": "and(job_name:*)",
        "lookup_key": "k8s_health_jobs",
        "metric_name": "kube_job_status_failed",
        "metric_type": "numeric",
        "name": "Kubernetes Job Failures ({cluster_name})",
        "rules": [
			{
				"wait": 0,
				"severity": 0,
				"windowing_duration": 300,
				"windowing_min_duration": 0,
				"criteria": "on absence",
				"windowing_function": null,
				"value": 900
        	},
            {
                "criteria": "max value",
                "severity": 1,
                "wait": 0,
                "windowing_duration": 300,
                "value": "0"
            }
        ]        
    },
    "persistent_volume_failures": {
        "filter": "and(phase:Failed)",
        "lookup_key": "k8s_health_pvols",
        "metric_name": "kube_persistentvolume_status_phase",
        "metric_type": "numeric",
        "name": "Kubernetes Persistent Volume Failures ({cluster_name})",
        "rules": [
			{
				"wait": 0,
				"severity": 0,
				"windowing_duration": 300,
				"windowing_min_duration": 0,
				"criteria": "on absence",
				"windowing_function": null,
				"value": 900
        	},
            {
                "criteria": "max value",
                "severity": 1,
                "wait": 0,
                "windowing_duration": 300,
                "value": "0"
            }
        ]        
    },
    "pod_pending_delays": {
        "filter": "and(phase:Pending)",
        "lookup_key": "k8s_health_poddelays",
        "metric_name": "kube_pod_status_phase",
        "metric_type": "numeric",
        "name": "Kubernetes Pod Pending Delays ({cluster_name})",
        "rules": [
			{
				"wait": 0,
				"severity": 0,
				"windowing_duration": 300,
				"windowing_min_duration": 0,
				"criteria": "on absence",
				"windowing_function": null,
				"value": 900
        	},
            {
                "criteria": "max value",
                "severity": 1,
                "wait": 0,
                "windowing_duration": 900,
				"windowing_function": "average",
				"windowing_min_duration": 900,
                "value": "0.99"
            }
        ]        
    },
    "deployment_glitches": {
        "filter": "and(deployment:*)",
        "lookup_key": "k8s_health_deploys",
        "metric_name": "deployment_generation_delta",
        "metric_type": "numeric",
        "name": "Kubernetes Deployment Glitches ({cluster_name})",
        "rules": [
			{
				"wait": 0,
				"severity": 0,
				"windowing_duration": 300,
				"windowing_min_duration": 0,
				"criteria": "on absence",
				"windowing_function": null,
				"value": 900
        	},
            {
                "criteria": "max value",
                "severity": 1,
                "wait": 0,
                "windowing_duration": 300,
                "value": "0"
            },
            {
                "criteria": "min value",
                "severity": 1,
                "wait": 0,
                "windowing_duration": 300,
                "value": "0"
            }
        ]        
    },
    "daemonsets_not_ready": {
        "filter": "and(daemonset:*)",
        "lookup_key": "k8s_health_daemonsets",
        "metric_name": "daemonset_scheduled_delta",
        "metric_type": "numeric",
        "name": "Kubernetes DaemonSets Not Ready ({cluster_name})",
        "rules": [
			{
				"wait": 0,
				"severity": 0,
				"windowing_duration": 300,
				"windowing_min_duration": 0,
				"criteria": "on absence",
				"windowing_function": null,
				"value": 900
        	},
            {
                "criteria": "max value",
                "severity": 1,
                "wait": 0,
                "windowing_duration": 300,
                "value": "0"
            },
            {
                "criteria": "min value",
                "severity": 1,
                "wait": 0,
                "windowing_duration": 300,
                "value": "0"
            }
        ]
    },
    "statefulsets_not_ready": {
        "filter": "and(statefulset:*)",
        "lookup_key": "k8s_health_statefulsets",
        "metric_name": "statefulset_replica_delta",
        "metric_type": "numeric",
        "name": "Kubernetes StatefulSets Not Ready ({cluster_name})",
        "rules": [
			{
				"wait": 0,
				"severity": 0,
				"windowing_duration": 300,
				"windowing_min_duration": 0,
				"criteria": "on absence",
				"windowing_function": null,
				"value": 900
        	},
            {
                "criteria": "max value",
                "severity": 1,
                "wait": 0,
                "windowing_duration": 300,
                "value": "0"
            },
            {
                "criteria": "min value",
                "severity": 1,
                "wait": 0,
                "windowing_duration": 300,
                "value": "0"
            }
        ]
    }
}
`

type DefaultAlerts struct {
	RuleSettings map[string]RuleSettings `json:"rule_settings"`
	Contact      AlertContact            `json:"contact"`
}

type AlertContact struct {
	Email    string `json:"email"`
	GroupCID string `json:"group_cid"`
}

type RuleSettings struct {
	Threshold    string `json:"threshold"`
	MinThreshold string `json:"min_threshold"`
	MaxThreshold string `json:"max_threshold"`
	MaxWindow    uint   `json:"max_window"`
	Window       uint   `json:"window"`
	MinWindow    uint   `json:"min_window"`
	Disabled     bool   `json:"disabled"`
}

type CustomRules struct {
	Rules []apiclient.RuleSet `json:"rules"`
}

func initializeAlerting(client *apiclient.API, logger zerolog.Logger, clusterName, clusterTag, clusterVers, checkCID, checkUUID string) {
	configFile := viper.GetString(keys.DefaultAlertsFile)
	data, err := ioutil.ReadFile(configFile)
	if err != nil {
		logger.Warn().Err(err).Str("alert_config", configFile).Msg("skipping")
		return
	}

	var da DefaultAlerts
	if err := json.Unmarshal(data, &da); err != nil {
		logger.Warn().Err(err).Msg("unable to parse alert config, skipping")
	}

	if da.Contact.Email == "" && da.Contact.GroupCID == "" {
		logger.Warn().Msg("no default alerting contact configured -- alerting is DISABLED")
		return
	}

	// find/create contact
	cg, err := createContact(client, logger, da, clusterName, clusterTag)
	if err != nil {
		logger.Error().Err(err).Msg("alerting contact")
		return
	}
	logger.Debug().Str("contact", cg.Name).Str("cid", cg.CID).Msg("using contact group")

	// create/update default rules
	if err := manageDefaultRules(client, logger, da, clusterName, clusterTag, clusterVers, checkCID, checkUUID, cg); err != nil {
		logger.Error().Err(err).Msg("alerting default rules")
		return
	}

	// create custom rules
	if err := createCustomRules(client, logger, clusterName, clusterTag, checkCID); err != nil {
		logger.Error().Err(err).Msg("alerting custom rules")
		return
	}
}

func createContact(client *apiclient.API, logger zerolog.Logger, da DefaultAlerts, clusterName, clusterTag string) (*apiclient.ContactGroup, error) {
	logger.Debug().Msg("fetch/create altering contact")

	// group_cid takes precedence as it is the most specific
	if da.Contact.GroupCID != "" {
		cid := da.Contact.GroupCID
		cg, err := client.FetchContactGroup(apiclient.CIDType(&cid))
		if err != nil {
			return nil, errors.Wrapf(err, "fetching contact group (cid:%s)", cid)
		}
		return cg, nil
	}

	cgName := clusterName + " default alerts"

	// find
	query := apiclient.SearchQueryType(cgName + " (active:1)(tags:" + clusterTag + ")")
	cgs, err := client.SearchContactGroups(&query, nil)
	if err != nil {
		return nil, errors.Wrapf(err, "searching for contact group (%s)", query)
	}
	if len(*cgs) > 0 {
		if len(*cgs) > 1 {
			return nil, fmt.Errorf("found multiple (%d) contact groups matching criteria (%s)", len(*cgs), query)
		}
		cg := (*cgs)[0]
		// verify default contact email is in the list of contacts - makes it so that a user "could" modify
		// the default group and add additional contacts without disrupting the "automated" default alerting
		foundContact := false
		for _, contact := range cg.Contacts.External {
			if contact.Info == da.Contact.Email {
				foundContact = true
			}
		}
		if foundContact {
			return &cg, nil
		}
		logger.Warn().Interface("cg", cg).Str("email", da.Contact.Email).Msg("found contact group, missing alert email, updating")
		cg.Contacts.External = append(cg.Contacts.External, apiclient.ContactGroupContactsExternal{
			Info:   da.Contact.Email,
			Method: "email",
		})
		ncg, err := client.UpdateContactGroup(&cg)
		if err != nil {
			return nil, errors.Wrapf(err, "updating contact group (%#v)", cg)
		}
		return ncg, nil
	}

	// create
	cfg := &apiclient.ContactGroup{
		Name: cgName,
		Contacts: apiclient.ContactGroupContacts{
			External: []apiclient.ContactGroupContactsExternal{
				{
					Info:   da.Contact.Email,
					Method: "email",
				},
			},
		},
		Tags: []string{clusterTag},
	}

	cg, err := client.CreateContactGroup(cfg)
	if err != nil {
		return nil, errors.Wrapf(err, "creating contact group (%#v)", cfg)
	}

	return cg, nil
}

func addTag(tags []string, newTag string) []string {
	tagList := map[string]bool{newTag: true}
	for _, tag := range tags {
		// if tag == "" {
		//     continue
		// }
		if _, found := tagList[tag]; !found {
			tagList[tag] = true
		}
	}
	newTagList := make([]string, len(tagList))
	i := 0
	for tag := range tagList {
		newTagList[i] = tag
		i++
	}
	sort.Strings(newTagList)
	return newTagList
}

func manageDefaultRules(client *apiclient.API, logger zerolog.Logger, da DefaultAlerts, clusterName, clusterTag, clusterVers, checkCID, checkUUID string, cg *apiclient.ContactGroup) error {
	logger.Debug().Msg("manage default alerting rules")
	rules, err := defaultRules(clusterVers)
	if err != nil {
		return errors.Wrap(err, "loading default rules")
	}

	for rid, ruleTemplate := range rules {
		ruleSettings, haveSettings := da.RuleSettings[rid]

		if ruleTemplate.Name == "" {
			ruleTemplate.Name = rid + " (" + clusterName + ")"
		} else {
			ruleTemplate.Name = strings.Replace(ruleTemplate.Name, "{cluster_name}", clusterName, 1)
		}

		ruleTemplate.CheckCID = checkCID
		ruleTemplate.Tags = addTag(ruleTemplate.Tags, clusterTag)

		ruleTemplate.ContactGroups = map[uint8][]string{
			1: {cg.CID},
			2: {},
			3: {},
			4: {},
			5: {},
		}

		note := release.NAME + " default rule for " + rid + ". NOTE: any changes (except contact groups) to default rules will be overwritten at next deployment."
		ruleTemplate.Notes = &note

		if haveSettings {
			logger.Debug().Str("rule_id", rid).Interface("settings", ruleSettings).Msg("applying settings")
			switch rid {
			case "cpu_utilization":
				v, err := strconv.Atoi(ruleSettings.Threshold)
				switch {
				case err != nil:
					logger.Warn().
						Err(err).
						Str("rule_id", rid).
						Str("threshold", ruleSettings.Threshold).
						Msg("invalid threshold, unable to parse, using default")
				case v < 1 || v > 99:
					logger.Warn().
						Str("rule_id", rid).
						Str("threshold", ruleSettings.Threshold).
						Msg("invalid threshold, acceptable 1-99, using default")
				default:
					ruleTemplate.Rules[1].Value = ruleSettings.Threshold
				}
				if ruleSettings.Window > 59 {
					ruleTemplate.Rules[1].WindowingDuration = ruleSettings.Window
				}
			case "deployment_glitches", "daemonsets_not_ready", "statefulsets_not_ready":
				if ruleSettings.MaxThreshold != "" {
					ruleTemplate.Rules[1].Value = ruleSettings.MaxThreshold
				}
				if ruleSettings.MaxWindow > 59 {
					ruleTemplate.Rules[1].WindowingDuration = ruleSettings.MaxWindow
				}
				if ruleSettings.MinThreshold != "" {
					ruleTemplate.Rules[2].Value = ruleSettings.MinThreshold
				}
				if ruleSettings.MinWindow > 59 {
					ruleTemplate.Rules[2].WindowingDuration = ruleSettings.MinWindow
				}
			default:
				if ruleSettings.Threshold != "" {
					ruleTemplate.Rules[1].Value = ruleSettings.Threshold
				}
				if ruleSettings.Window > 59 {
					ruleTemplate.Rules[1].WindowingDuration = ruleSettings.Window
				}
			}
		}

		//
		// search for existing rule
		//
		query := apiclient.SearchQueryType(fmt.Sprintf(`(name:"%s")(active_check:1)(check_uuid:"%s")(tags:%s)`, ruleTemplate.Name, checkUUID, clusterTag))
		existingRules, err := client.SearchRuleSets(&query, nil)
		if err != nil {
			logger.Error().Err(err).Str("query", string(query)).Msg("searching, skipping rule")
			continue
		}
		if len(*existingRules) == 0 {
			if haveSettings && ruleSettings.Disabled {
				logger.Warn().Str("rule_id", rid).Msg("disabled, skipping")
				continue
			}
			logger.Debug().Str("rule_id", rid).Msg("creating rule")
			if err := makeRule(client, logger, ruleTemplate, true); err != nil {
				logger.Warn().Err(err).Str("rule_id", rid).Interface("rule_cfg", ruleTemplate).Msg("creating rule")
			}
			continue
		}

		if len(*existingRules) > 1 {
			logger.Error().Str("query", string(query)).Int("num_rules", len(*existingRules)).Interface("rules", *existingRules).Msg("more than one rule matching search criteria found")
			continue
		}

		update := false
		delete := false

		origRule := (*existingRules)[0]

		ruleTemplate.CID = origRule.CID
		ruleTemplate.Host = origRule.Host
		ruleTemplate.Tags = addTag(origRule.Tags, clusterTag)
		if len(origRule.ContactGroups) > 0 {
			for sevLevel, cgList := range origRule.ContactGroups {
				if sevLevel != 1 {
					continue
				}
				add := true
				for _, cgCID := range cgList {
					if cgCID == cg.CID {
						add = false
						break
					}
				}
				if add {
					origRule.ContactGroups[sevLevel] = append(origRule.ContactGroups[sevLevel], cg.CID)
					break
				}
			}
			ruleTemplate.ContactGroups = origRule.ContactGroups
		}

		// if the metric criteria change, a new rule will be created even on a PUT (update)
		// mark the original to be deleted if any of these criteria are true
		switch {
		case origRule.MetricPattern != ruleTemplate.MetricPattern:
			delete = true
		case origRule.MetricName != ruleTemplate.MetricName:
			delete = true
		case origRule.Filter != ruleTemplate.Filter:
			delete = true
		}

		// delete disabled rule if found in search
		if haveSettings && ruleSettings.Disabled {
			logger.Warn().Str("rule_id", rid).Msg("disabled, removing old ruleset")
			delete = true
		}

		if delete {
			// empty out the private fields
			ruleTemplate.CID = ""
			ruleTemplate.Host = ""
			logger.Debug().Str("rule_id", rid).Msg("creating rule")
			if err := makeRule(client, logger, ruleTemplate, true); err != nil {
				logger.Warn().Err(err).Str("rule_id", rid).Interface("rule_cfg", ruleTemplate).Msg("creating rule")
				continue // if there's an error creating, leave the old rule in place
			}
			logger.Debug().Str("rule_id", rid).Str("cid", origRule.CID).Msg("deleting old rule")
			if _, err := client.DeleteRuleSetByCID(apiclient.CIDType(&origRule.CID)); err != nil {
				logger.Warn().Err(err).Str("rule_id", rid).Interface("rule_cfg", origRule).Msg("deleting old rule")
			}
			continue
		}

		origData, err := json.Marshal(origRule)
		if err != nil {
			logger.Warn().Err(err).Interface("orig_rule", origRule).Msg("encoding original rule")
			continue
		}
		mergedData, err := json.Marshal(ruleTemplate)
		if err != nil {
			logger.Warn().Err(err).Interface("merged_rule", ruleTemplate).Msg("encoding merged rule")
			continue
		}

		if string(origData) != string(mergedData) {
			logger.Debug().RawJSON("orig", origData).RawJSON("merged", mergedData).Msg("not the same, updating rule")
			update = true
		}

		if update {
			logger.Debug().Str("rule_id", rid).Msg("updating rule")
			if err := makeRule(client, logger, ruleTemplate, false); err != nil {
				logger.Warn().Err(err).Str("rule_id", rid).Interface("rule_cfg", ruleTemplate).Msg("updating rule")
			}
		} else {
			logger.Debug().Str("rule_id", rid).Msg("unchanged, no update")
		}
	}

	return nil
}

func createCustomRules(client *apiclient.API, logger zerolog.Logger, clusterName, clusterTag, checkCID string) error {
	logger.Debug().Msg("create custom alerting rules")
	configFile := viper.GetString(keys.CustomRulesFile)
	data, err := ioutil.ReadFile(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		logger.Warn().Err(err).Str("custom_rule_config", configFile).Msg("loading")
		return err
	}

	var cr CustomRules
	if err := json.Unmarshal(data, &cr); err != nil {
		return err
	}

	create := true

	// "managing" custom rules is more complex. for the moment, we'll rely on API to return
	// an existing ruleset if the config matches.
	for _, rule := range cr.Rules {
		// create custom rules from configuration
		if rule.CheckCID == "" {
			rule.CheckCID = checkCID
		}
		rule.Name = strings.Replace(rule.Name, "{cluster_name}", clusterName, 1)
		rule.Tags = append(rule.Tags, clusterTag)
		if err := makeRule(client, logger, rule, create); err != nil {
			return err
		}
	}

	return nil
}

func makeRule(client *apiclient.API, logger zerolog.Logger, rule apiclient.RuleSet, create bool) error {

	if create {
		r, err := client.CreateRuleSet(&rule)
		if err != nil {
			return err
		}
		logger.Debug().Interface("rule", r).Msg("created rule set")
	} else {
		r, err := client.UpdateRuleSet(&rule)
		if err != nil {
			return err
		}
		logger.Debug().Interface("rule", r).Msg("updated rule set")
	}

	return nil
}

func defaultRules(clusterVers string) (map[string]apiclient.RuleSet, error) {
	var defaultRuleSetsData []byte

	currversion, err := version.NewVersion(clusterVers)
	if err != nil {
		return nil, err
	}

	v120, err := version.NewVersion("v1.20")
	if err != nil {
		return nil, err
	}

	if currversion.LessThan(v120) {
		defaultRuleSetsData = []byte(defaultRuleSetsStr119)
	} else {
		defaultRuleSetsData = []byte(defaultRuleSetsStr120)
	}

	var rules map[string]apiclient.RuleSet
	if err := json.Unmarshal(defaultRuleSetsData, &rules); err != nil {
		return nil, err
	}
	return rules, nil
}
