package config

import (
	"fmt"
	"strings"
)

type KernelCmdLine map[string]string

func (k *KernelCmdLine) Set(key, value string) {
	(*k)[key] = value
}

func (k *KernelCmdLine) String() string {
	output := []string{}

	for key, value := range *k {
		if value == "" {
			output = append(output, key)
		} else {
			output = append(output, fmt.Sprintf("%s=%s", key, value))
		}
	}

	return strings.Join(output, " ")
}
