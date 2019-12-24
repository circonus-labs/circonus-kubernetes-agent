// Copyright © 2019 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package k8s

type ServiceList struct {
	Items []*Service `json:"items"`
}
type Service struct {
	Metadata ServiceMetadata `json:"metadata"`
	Spec     ServiceSpec     `json:"spec"`
}
type ServiceMetadata struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	SelfLink  string `json:"selfLink"`
}
type ServiceSpec struct {
	Ports []ServicePort `json:"ports"`
}
type ServicePort struct {
	Name       string `json:"name"`
	Protocol   string `json:"protocol"`
	Port       uint   `json:"port"`
	TargetPort string `json:"targetPort"`
}
