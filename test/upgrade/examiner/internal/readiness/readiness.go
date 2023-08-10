/*
Copyright 2019 The Kubernetes Authors.

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

package readiness

import (
	"context"
	"fmt"
	"time"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

const (
	waitInterval    = 5 * time.Second
	timeoutInterval = 120 * time.Second
)

// ClientGetter is an interface to represent structs return kubernetes clientset
type ClientGetter interface {
	KubernetesClient() kubernetes.Interface
}

type readiness struct {
	client kubernetes.Interface
	cfg    ServiceCatalogConfig
}

// NewReadiness returns pointer to rediness probe
func NewReadiness(c ClientGetter, scConfig ServiceCatalogConfig) *readiness {
	return &readiness{
		client: c.KubernetesClient(),
		cfg:    scConfig,
	}
}

// TestEnvironmentIsReady runs probe to check all required pods are running
func (r *readiness) TestEnvironmentIsReady() error {
	klog.Info("Assert all pods required to test are ready")
	for _, fn := range []func() error{
		r.assertServiceCatalogIsReady,
		r.assertTestBrokerIsReady,
	} {
		err := fn()
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *readiness) assertServiceCatalogIsReady() error {
	klog.Info("Make sure ServiceCatalog ApiServer is up")
	if err := r.assertServiceCatalogAPIServerIsUp(); err != nil {
		return fmt.Errorf("failed during waiting for ServiceCatalog ApiServer: %w", err)
	}
	klog.Info("ServiceCatalog ApiServer is ready")

	klog.Info("Make sure ServiceCatalog Controller is up")
	if err := r.assertServiceCatalogControllerIsUp(); err != nil {
		return fmt.Errorf("failed during waiting for ServiceCatalog Controller: %w", err)
	}
	klog.Info("ServiceCatalog Controller is ready")

	return nil
}

func (r *readiness) assertServiceCatalogAPIServerIsUp() error {
	return wait.PollUntilContextTimeout(context.Background(), waitInterval, timeoutInterval, false, func(context.Context) (done bool, err error) {
		deployment, err := r.client.AppsV1().Deployments(r.cfg.ServiceCatalogNamespace).Get(context.Background(), r.cfg.ServiceCatalogAPIServerName, v1.GetOptions{})
		if err != nil {
			return false, err
		}
		ready := deployment.Status.ReadyReplicas
		available := deployment.Status.AvailableReplicas
		if ready >= 1 && available >= 1 {
			return true, nil
		}
		return false, nil
	})
}

func (r *readiness) assertServiceCatalogControllerIsUp() error {
	return wait.PollUntilContextTimeout(context.Background(), waitInterval, timeoutInterval, false, func(context.Context) (done bool, err error) {
		deployment, err := r.client.AppsV1().Deployments(r.cfg.ServiceCatalogNamespace).Get(context.Background(), r.cfg.ServiceCatalogControllerServerName, v1.GetOptions{})
		if err != nil {
			return false, err
		}
		ready := deployment.Status.ReadyReplicas
		available := deployment.Status.AvailableReplicas
		if ready >= 1 && available >= 1 {
			return true, nil
		}
		return false, nil
	})
}

func (r *readiness) assertTestBrokerIsReady() error {
	klog.Info("Make sure TestBroker is up")
	if err := r.assertTestBrokerIsUp(); err != nil {
		return fmt.Errorf("failed during waiting for TestBroker: %w", err)
	}
	klog.Info("TestBroker is ready")

	return nil
}

func (r *readiness) assertTestBrokerIsUp() error {
	return wait.PollUntilContextTimeout(context.Background(), waitInterval, timeoutInterval, false, func(context.Context) (done bool, err error) {
		deployment, err := r.client.AppsV1().Deployments(r.cfg.TestBrokerNamespace).Get(context.Background(), r.cfg.TestBrokerName, v1.GetOptions{})
		if err != nil {
			return false, err
		}
		ready := deployment.Status.ReadyReplicas
		available := deployment.Status.AvailableReplicas
		if ready >= 1 && available >= 1 {
			return true, nil
		}
		return false, nil
	})
}
