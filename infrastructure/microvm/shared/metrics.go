package shared

import (
	"fmt"
	"strings"
)

type MachineMetrics struct {
	Namespace   string  `json:"Namespace"`
	MachineName string  `json:"MachineName"`
	MachineUID  string  `json:"MachineUID"`
	Data        Metrics `json:"Data"`
}

type Metrics map[string]map[string]int64

func (mm MachineMetrics) ToPrometheus() []byte {
	output := []string{}
	labels := strings.Join(
		[]string{
			metricsLabel("namespace", mm.Namespace),
			metricsLabel("name", mm.MachineName),
			metricsLabel("uid", mm.MachineUID),
		},
		",",
	)

	for prefix, group := range mm.Data {
		for key, value := range group {
			output = append(output, fmt.Sprintf("%s_%s{%s} %d", prefix, key, labels, value))
		}
	}

	return []byte(strings.Join(output, "\n"))
}

func metricsLabel(key, value string) string {
	return fmt.Sprintf("%s=\"%s\"", key, value)
}
