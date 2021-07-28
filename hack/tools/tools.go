// +build tools

package tools

import (
	_ "github.com/golang/mock/mockgen"
	_ "github.com/golangci/golangci-lint/cmd/golangci-lint"
	_ "github.com/onsi/ginkgo/ginkgo"
	_ "k8s.io/code-generator"
	_ "k8s.io/code-generator/cmd/conversion-gen"
	_ "sigs.k8s.io/controller-tools/cmd/controller-gen"
)
