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

package migration

import (
	"fmt"

	"github.com/spf13/pflag"
)

const (
	backupActionName          = "backup"
	restoreActionName         = "restore"
	deployBlockerActionName   = "deploy-blocker"
	undeployBlockerActionName = "undeploy-blocker"

	storagePathParameter             = "storage-path"
	apiserverNameParameter           = "apiserver-deployment"
	controllerManagerNameParameter   = "controller-manager-deployment"
	serviceCatalogNamespaceParameter = "service-catalog-namespace"
	webhookServiceNameParameter      = "webhook-service-name"
	webhookServicePortParameter      = "webhook-service-port"
	pvcNameParameter                 = "pvc-name"
)

// Options holds configuration for the migration job
type Options struct {
	Action                    string
	StoragePath               string
	ReleaseNamespace          string
	ControllerManagerName     string
	ApiserverName             string
	WebhookServiceName        string
	WebhookServicePort        string
	PersistentVolumeClaimName string
}

// NewMigrationOptions creates and returns a new Options
func NewMigrationOptions() *Options {
	return &Options{}
}

// AddFlags adds flags for a CleanerOptions to the specified FlagSet.
func (c *Options) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&c.Action, "action", "", "Command name to execute")
	fs.StringVar(&c.StoragePath, storagePathParameter, "", "Path to a directory, where all Service Catalog resources must be saved")
	fs.StringVar(&c.ReleaseNamespace, serviceCatalogNamespaceParameter, "", "Name of namespace where Service Catalog is released")
	fs.StringVar(&c.ControllerManagerName, controllerManagerNameParameter, "", "Name of controller manager deployment")
	fs.StringVar(&c.ApiserverName, apiserverNameParameter, "", "Name of apiserver deployment")
	fs.StringVar(&c.WebhookServiceName, webhookServiceNameParameter, "", "Name of webhook service")
	fs.StringVar(&c.WebhookServicePort, webhookServicePortParameter, "", "Port of the webhook service")
	fs.StringVar(&c.PersistentVolumeClaimName, pvcNameParameter, "", "Name of PersistentVolumeClaim in which resources will be stored")
}

// Validate checks flag has been set and has a proper value
func (c *Options) Validate() error {
	switch c.Action {
	case backupActionName:
		if err := c.requiredParameters(map[string]string{
			serviceCatalogNamespaceParameter: c.ReleaseNamespace,
			controllerManagerNameParameter:   c.ControllerManagerName,
			apiserverNameParameter:           c.ApiserverName,
			storagePathParameter:             c.StoragePath,
		}); err != nil {
			return err
		}
	case restoreActionName:
		if err := c.requiredParameters(map[string]string{
			serviceCatalogNamespaceParameter: c.ReleaseNamespace,
			controllerManagerNameParameter:   c.ControllerManagerName,
			webhookServiceNameParameter:      c.WebhookServiceName,
			webhookServicePortParameter:      c.WebhookServicePort,
			storagePathParameter:             c.StoragePath,
			pvcNameParameter:                 c.PersistentVolumeClaimName,
		}); err != nil {
			return err
		}
	case deployBlockerActionName, undeployBlockerActionName:
		return nil
	default:
		return fmt.Errorf("action must be 'restore', 'backup', 'deploy-blocker' or 'undeploy-blocker', you provided %s", c.Action)
	}

	return nil
}

func (c *Options) requiredParameters(parameters map[string]string) error {
	for name, value := range parameters {
		if value == "" {
			return fmt.Errorf("%s must not be empty", name)
		}
	}

	return nil
}
