// Copyright © 2019 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package k8s

type NodeList struct {
	Items []Node `json:"items"`
}

type Node struct {
	Metadata NodeMetadata `json:"metadata"`
	Status   NodeStatus   `json:"status"`
}

type NodeMetadata struct {
	Name     string            `json:"name"`
	SelfLink string            `json:"selfLink"`
	Labels   map[string]string `json:"labels"`
}

type NodeStatus struct {
	Conditions  []NodeCondition `json:"conditions"`
	NodeInfo    NodeInfo        `json:"nodeInfo"`
	Capacity    NodeSizes       `json:"capacity"`
	Allocatable NodeSizes       `json:"allocatable"`
}

type NodeSizes struct {
	CPU    string `json:"cpu"`
	Memory string `json:"memory"`
	Pods   string `json:"pods"`
}

type NodeCondition struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type NodeInfo struct {
	KernelVersion  string `json:"kernelVersion"`
	OSImage        string `json:"osImage"`
	KubeletVersion string `json:"kubeletVersion"`
}
