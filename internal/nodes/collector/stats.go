// Copyright © 2020 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package collector

import (
	"sync"

	v1 "k8s.io/api/core/v1"
)

type NodeStat struct {
	CPUCapacity        uint64
	LastCPUNanoSeconds uint64
	Conditions         map[v1.NodeConditionType]v1.ConditionStatus
}

var (
	nodeStats   map[string]NodeStat
	nodeStatsmu sync.Mutex
)

func init() {
	nodeStats = make(map[string]NodeStat)
}

func NewNodeStat() NodeStat {
	return NodeStat{
		Conditions: make(map[v1.NodeConditionType]v1.ConditionStatus),
	}
}

func SetNodeStat(nodeName string, stat NodeStat) {
	nodeStatsmu.Lock()
	nodeStats[nodeName] = stat
	nodeStatsmu.Unlock()
}

func GetNodeStat(nodeName string) (NodeStat, bool) {
	nodeStatsmu.Lock()
	ns, ok := nodeStats[nodeName]
	nodeStatsmu.Unlock()
	return ns, ok
}
