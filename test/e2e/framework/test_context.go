/*
Copyright 2016 The Kubernetes Authors.

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

package framework

import (
	"flag"
	"os"

	"k8s.io/client-go/tools/clientcmd"
)

const (
	RecommendedConfigPathEnvVar = "SERVICECATALOGCONFIG"
)

type TestContextType struct {
	BrokerImage           string
	KubeHost              string
	KubeConfig            string
	KubeContext           string
	ServiceCatalogHost    string
	ServiceCatalogConfig  string
	ServiceCatalogContext string
}

var TestContext TestContextType

// Register flags common to all e2e test suites.
func RegisterCommonFlags() {

	flag.StringVar(&TestContext.BrokerImage, "broker-image", "quay.io/kubernetes-service-catalog/user-broker:latest",
		"The container image for the broker to test against")
	flag.StringVar(&TestContext.KubeHost, "kubernetes-host", "http://127.0.0.1:8080", "The kubernetes host, or apiserver, to connect to")
	flag.StringVar(&TestContext.KubeConfig, "kubernetes-config", os.Getenv(clientcmd.RecommendedConfigPathEnvVar), "Path to config containing embedded authinfo for kubernetes. Default value is from environment variable "+clientcmd.RecommendedConfigPathEnvVar)
	flag.StringVar(&TestContext.KubeContext, "kubernetes-context", "", "config context to use for kubernetes. If unset, will use value from 'current-context'")
	flag.StringVar(&TestContext.ServiceCatalogHost, "service-catalog-host", "http://127.0.0.1:30000", "The service catalog host, or apiserver, to connect to")
	flag.StringVar(&TestContext.ServiceCatalogConfig, "service-catalog-config", os.Getenv(RecommendedConfigPathEnvVar), "Path to config containing embedded authinfo for service catalog. Default value is from environment variable "+RecommendedConfigPathEnvVar)
	flag.StringVar(&TestContext.ServiceCatalogContext, "service-catalog-context", "", "config context to use for service catalog. If unset, will use value from 'current-context'")
}

func RegisterParseFlags() {
	RegisterCommonFlags()
	flag.Parse()
}
