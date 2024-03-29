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

// The controller is responsible for running control loops that reconcile
// the state of service catalog API resources with service brokers, service
// classes, service instances, and service bindings.

package componentconfig

import (
	"time"

	"github.com/drycc-addons/service-catalog/pkg/kubernetes/pkg/apis/componentconfig"
	genericoptions "k8s.io/apiserver/pkg/server/options"
)

// ControllerManagerConfiguration encapsulates configuration for the
// controller manager.
type ControllerManagerConfiguration struct {
	// DEPRECATED/Ignored, use SecureServingOptions.BindAddress instead.
	Address string

	// DEPRECATED/Ignored, use SecureServingOptions.SecurePort instead.
	Port int32

	// ContentType is the content type for requests sent to API servers.
	ContentType string

	// kubeAPIQPS is the QPS to use while talking with kubernetes apiserver.
	KubeAPIQPS float32
	// kubeAPIBurst is the burst to use while talking with kubernetes apiserver.
	KubeAPIBurst int32

	// K8sAPIServerURL is the URL for the k8s API server.
	K8sAPIServerURL string
	// K8sKubeconfigPath is the path to the kubeconfig file with authorization
	// information.
	K8sKubeconfigPath string

	// ServiceCatalogAPIServerURL is the URL for the service-catalog API
	// server.
	ServiceCatalogAPIServerURL string
	// ServiceCatalogKubeconfigPath is the path to the kubeconfig file with
	// information about the service catalog API server.
	ServiceCatalogKubeconfigPath string
	// InsecureSkipVerify controls whether a client verifies the
	// server's certificate chain and host name.
	// If InsecureSkipVerify is true, TLS accepts any certificate
	// presented by the server and any host name in that certificate.
	// In this mode, TLS is susceptible to man-in-the-middle attacks.
	// This should be used only for testing.
	ServiceCatalogInsecureSkipVerify bool

	// ResyncInterval is the interval on which the controller should re-sync
	// all informers.
	ResyncInterval time.Duration

	// ServiceBrokerRelistInterval is the interval on which Broker's catalogs are re-
	// listed.
	ServiceBrokerRelistInterval time.Duration

	// Whether or not to send the proposed optional
	// OpenServiceBroker API Context Profile field
	OSBAPIContextProfile   bool
	OSBAPIPreferredVersion string

	// OSBAPITimeOut the length of the timeout of any request to the broker.
	OSBAPITimeOut time.Duration

	// ConcurrentSyncs is the number of resources, per resource type,
	// that are allowed to sync concurrently. Larger number = more responsive
	// SC operations, but more CPU (and network) load.
	ConcurrentSyncs int

	// leaderElection defines the configuration of leader election client.
	LeaderElection componentconfig.LeaderElectionConfiguration

	// LeaderElectionNamespace is the namespace to use for the leader election
	// lock.
	LeaderElectionNamespace string

	// enableProfiling enables profiling via web interface host:port/debug/pprof/
	EnableProfiling bool

	// enableContentionProfiling enables lock contention profiling, if enableProfiling is true.
	EnableContentionProfiling bool

	// ReconciliationRetryDuration is the longest time to attempt reconciliation
	// on a given resource before failing the reconciliation
	ReconciliationRetryDuration time.Duration

	// OperationPollingMaximumBackoffDuration is the maximum duration that exponential
	// backoff for polling OSB API operations will use.
	OperationPollingMaximumBackoffDuration time.Duration

	SecureServingOptions *genericoptions.SecureServingOptions

	// ClusterIDConfigMapName is the k8s name that the clusterid configmap will have
	ClusterIDConfigMapName string
	// ClusterIDConfigMapNamespace is the k8s namespace that the clusterid configmap will be stored in.
	ClusterIDConfigMapNamespace string
}
