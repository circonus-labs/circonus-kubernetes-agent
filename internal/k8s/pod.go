// Copyright © 2019 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package k8s

type PodList struct {
	Items []*Pod `json:"items"`
}
type Pod struct {
	Metadata PodMetadata `json:"metadata"`
	Spec     PodSpec     `json:"spec"`
}
type PodMetadata struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	SelfLink  string `json:"selfLink"`
}
type PodSpec struct {
	Status PodStatus `json:"status"`
}
type PodStatus struct {
	PodIP string `json:"podIP"`
}
