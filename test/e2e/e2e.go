/*
Copyright 2015 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package e2e

import (
	"testing"

	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/component-base/logs"
	"k8s.io/klog/v2"

	"github.com/drycc-addons/service-catalog/test/e2e/framework"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

// RunE2ETests checks configuration parameters (specified through flags) and then runs
// E2E tests using the Ginkgo runner.
func RunE2ETests(t *testing.T) {
	logs.InitLogs()
	defer logs.FlushLogs()

	gomega.RegisterFailHandler(ginkgo.Fail)

	suiteConfig, reporterConfig := ginkgo.GinkgoConfiguration()
	// adjust it
	suiteConfig.EmitSpecProgress = true
	suiteConfig.RandomizeAllSpecs = true
	suiteConfig.SkipStrings = []string{`\[Flaky\]`, `\[Feature:.+\]`}

	reporterConfig.Verbose = true
	reporterConfig.FullTrace = true

	klog.Infof("Starting e2e run %q on Ginkgo host %s", framework.RunId, suiteConfig.ParallelHost)
	ginkgo.RunSpecs(t, "Service Catalog e2e suite", suiteConfig, reporterConfig)
}
