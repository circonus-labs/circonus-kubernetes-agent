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
	"strings"

	"github.com/circonus-labs/circonus-kubernetes-agent/internal/config/keys"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/release"
	apiclient "github.com/circonus-labs/go-apiclient"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"
)

type DefaultAlerts struct {
	Contact      AlertContact            `json:"contact"`
	RuleSettings map[string]RuleSettings `json:"rule_settings"`
}

type AlertContact struct {
	Email    string `json:"email"`
	GroupCID string `json:"group_cid"`
}

type RuleSettings struct {
	Threshold string `json:"threshold"`
	Window    uint   `json:"window"`
}

type CustomRules struct {
	Rules []apiclient.RuleSet `json:"rules"`
}

func initializeAlerting(client *apiclient.API, logger zerolog.Logger, clusterName, checkCID string) {
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
	cg, err := createContact(client, logger, da, clusterName)
	if err != nil {
		logger.Error().Err(err).Msg("alerting contact")
		return
	}

	// create/update default rules
	if err := manageDefaultRules(client, logger, da, clusterName, checkCID, cg); err != nil {
		logger.Error().Err(err).Msg("alerting default rules")
		return
	}

	// create custom rules
	if err := createCustomRules(client, logger, clusterName, checkCID); err != nil {
		logger.Error().Err(err).Msg("alerting custom rules")
		return
	}
}

func createContact(client *apiclient.API, logger zerolog.Logger, da DefaultAlerts, clusterName string) (*apiclient.ContactGroup, error) {
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
	cgTag := "cluster:" + clusterName

	// find
	query := apiclient.SearchQueryType(cgName + " (active:1)(tags:" + cgTag + ")")
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
		Tags: []string{cgTag},
	}

	cg, err := client.CreateContactGroup(cfg)
	if err != nil {
		return nil, errors.Wrapf(err, "creating contact group (%#v)", cfg)
	}

	return cg, nil
}

func manageDefaultRules(client *apiclient.API, logger zerolog.Logger, da DefaultAlerts, clusterName, checkCID string, cg *apiclient.ContactGroup) error {
	logger.Debug().Msg("manage default alerting rules")
	// create default rules with settings from configuration
	rules, err := defaultRules()
	if err != nil {
		return errors.Wrap(err, "loading default rules")
	}
	for rid, rule := range rules {
		if rule.Name == "" {
			rule.Name = rid + " (" + clusterName + ")"
		} else {
			rule.Name = strings.Replace(rule.Name, "{cluster_name}", clusterName, 1)
		}
		rule.CheckCID = checkCID
		rule.ContactGroups = map[uint8][]string{
			1: {cg.CID},
			2: {},
			3: {},
			4: {},
			5: {},
		}
		rule.Tags = []string{"cluster:" + clusterName}
		note := release.NAME + " v" + release.VERSION
		rule.Notes = &note
		switch rid {
		case "cpu_utilization":
			if settings, found := da.RuleSettings["cpu_utilization"]; found {
				rule.Rules[0].Value = settings.Threshold
				rule.Rules[0].WindowingDuration = settings.Window
			}
		case "pod_pending_delays":
			if settings, found := da.RuleSettings["pod_pending_delays"]; found {
				rule.Rules[0].WindowingDuration = settings.Window
			}
		}
		logger.Debug().Str("rule_id", rid).Msg("creating/updating rule")
		if err := makeRule(client, logger, rule); err != nil {
			logger.Warn().Err(err).Str("rule_id", rid).Interface("rule_cfg", rule).Msg("creating/updating rule")
		}
	}
	return nil
}

func createCustomRules(client *apiclient.API, logger zerolog.Logger, clusterName, checkCID string) error {
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

	for _, rule := range cr.Rules {
		// create custom rules from configuration
		if rule.CheckCID == "" {
			rule.CheckCID = checkCID
		}
		rule.Tags = append(rule.Tags, "cluster:"+clusterName)
		if err := makeRule(client, logger, rule); err != nil {
			return err
		}
	}

	return nil
}

func makeRule(client *apiclient.API, logger zerolog.Logger, rule apiclient.RuleSet) error {
	r, err := client.CreateRuleSet(&rule)
	if err != nil {
		return err
	}

	logger.Debug().Interface("rule", r).Msg("rule set")

	return nil
}

func defaultRules() (map[string]apiclient.RuleSet, error) {
	defaultRuleSetsData := []byte(`
{
    "crashloops": {
        "derive": null,
        "filter": "and(reason:CrashLoopBackOff)",
        "metric_pattern": "^kube_pod.*container_status_waiting_reason$",
        "metric_type": "numeric",
        "name": "Kubernetes CrashLoops ({cluster_name})",
        "rules": [
            {
                "wait": 0,
                "severity": 1,
                "windowing_function": null,
                "value": "0",
                "criteria": "max value",
                "windowing_duration": 300
            }
        ]
    },
    "cpu_utilization": {
        "derive": "average",
        "filter": "and(resource:cpu)",
        "metric_pattern": "^utilization$",
        "metric_type": "numeric",
        "name": "Kubernetes CPU ({cluster_name})",
        "rules": [
            {
                "windowing_function": "average",
                "severity": 1,
                "wait": 0,
                "windowing_duration": 900,
                "value": "60",
                "criteria": "max value"
            }
        ]
    },
    "disk_pressure": {
        "derive": null,
        "filter": "and(condition:DiskPressure,status:true)",
        "metric_pattern": "^kube_node_status_condition$",
        "metric_type": "numeric",
        "name": "Kubernetes Disk Pressure ({cluster_name})",
        "rules": [
            {
                "wait": 0,
                "severity": 1,
                "windowing_function": null,
                "criteria": "max value",
                "value": "0",
                "windowing_duration": 300
            }
        ]
    },
    "memory_pressure": {
        "derive": null,
        "filter": "and(condition:MemoryPressure,status:true)",
        "metric_pattern": "^kube_node_status_condition$",
        "metric_type": "numeric",
        "name": "Kubernetes Memory Pressure ({cluster_name})",
        "rules": [
            {
                "windowing_function": null,
                "severity": 1,
                "wait": 0,
                "windowing_duration": 300,
                "criteria": "max value",
                "value": "0"
            }
        ]
    },
    "pid_pressure": {
        "derive": null,
        "filter": "and(condition:PIDPressure,status:true)",
        "metric_pattern": "^kube_node_status_condition$",
        "metric_type": "numeric",
        "name": "Kubernetes PID Pressure ({cluster_name})",
        "rules": [
            {
                "severity": 1,
                "windowing_function": null,
                "wait": 0,
                "windowing_duration": 300,
                "value": "0",
                "criteria": "max value"
            }
        ]
    },
    "network_unavailable": {
        "derive": null,
        "filter": "and(condition:NetworkUnavailable,status:true)",
        "metric_pattern": "^kube_node_status_condition$",
        "metric_type": "numeric",
        "name": "Kubernetes Network Unavailable ({cluster_name})",
        "rules": [
            {
                "severity": 1,
                "windowing_function": null,
                "wait": 0,
                "windowing_duration": 300,
                "value": "0",
                "criteria": "max value"
            }
        ]
    },
    "job_failures": {
        "derive": null,
        "metric_pattern": "^kube_job_status_failed$",
        "metric_type": "numeric",
        "name": "Kubernetes Job Failures ({cluster_name})",
        "rules": [
            {
                "windowing_duration": 300,
                "value": "0",
                "criteria": "max value",
                "severity": 1,
                "windowing_function": null,
                "wait": 0
            }
        ]
    },
    "persistent_volume_failures": {
        "derive": null,
        "filter": "and(phase:Failed)",
        "metric_pattern": "^kube_persistentvolume_status_phase$",
        "metric_type": "numeric",
        "name": "Kubernetes Persistent Volume Failures ({cluster_name})",
        "rules": [
            {
                "criteria": "max value",
                "value": "0",
                "windowing_duration": 300,
                "wait": 0,
                "windowing_function": null,
                "severity": 1
            }
        ]
    },
    "pod_pending_delays": {
        "derive": "average",
        "filter": "and(phase:Pending)",
        "metric_pattern": "^kube_pod_status_phase$",
        "metric_type": "numeric",
        "name": "Kubernetes Pod Pending Delays ({cluster_name})",
        "rules": [
            {
                "severity": 1,
                "windowing_function": "average",
                "wait": 0,
                "windowing_duration": 900,
                "value": "0.99",
                "criteria": "max value"
            }
        ]
    },
    "deployment_glitches": {
        "derive": null,
        "metric_pattern": "^deployment_generation_delta$",
        "metric_type": "numeric",
        "name": "Kubernetes Deployment Glitches ({cluster_name})",
        "rules": [
            {
                "criteria": "max value",
                "value": "0",
                "windowing_duration": 300,
                "wait": 0,
                "windowing_function": null,
                "severity": 1
            },
            {
                "severity": 1,
                "windowing_function": null,
                "wait": 0,
                "windowing_duration": 300,
                "criteria": "min value",
                "value": "0"
            }
        ]
    },
    "daemonsets_not_ready": {
        "derive": null,
        "metric_pattern": "^daemonset_scheduled_delta$",
        "metric_type": "numeric",
        "name": "Kubernetes DaemonSets Not Ready ({cluster_name})",
        "rules": [
            {
                "windowing_function": null,
                "severity": 1,
                "wait": 0,
                "windowing_duration": 300,
                "criteria": "max value",
                "value": "0"
            },
            {
                "wait": 0,
                "severity": 1,
                "windowing_function": null,
                "criteria": "min value",
                "value": "0",
                "windowing_duration": 300
            }
        ]
    },
    "statefulsets_not_ready": {
        "derive": null,
        "metric_pattern": "^statefulset_replica_delta$",
        "metric_type": "numeric",
        "name": "Kubernetes StatefulSets Not Ready ({cluster_name})",
        "rules": [
            {
                "severity": 1,
                "windowing_function": null,
                "wait": 0,
                "windowing_duration": 300,
                "value": "0",
                "criteria": "max value"
            },
            {
                "criteria": "min value",
                "value": "0",
                "windowing_duration": 300,
                "wait": 0,
                "severity": 1,
                "windowing_function": null
            }
        ]
    }
}
`)

	var rules map[string]apiclient.RuleSet
	if err := json.Unmarshal(defaultRuleSetsData, &rules); err != nil {
		return nil, err
	}
	return rules, nil
}
