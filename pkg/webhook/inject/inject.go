/*
Copyright 2018 The Kubernetes Authors.

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

// Package inject is used by a Manager to inject types into Sources, EventHandlers, Predicates, and Reconciles.
// Deprecated: Use manager.Options fields directly. This package will be removed in v0.10.
package inject

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// ClientInjector is used by the ControllerManager to inject client into Sources, EventHandlers, Predicates, and
// Reconciles.
type ClientInjector interface {
	InjectClient(client.Client) error
}

// ClientInto will set client on i and return the result if it implements Client. Returns
// false if i does not implement Client.
func ClientInto(client client.Client, i interface{}) (bool, error) {
	if s, ok := i.(ClientInjector); ok {
		return true, s.InjectClient(client)
	}
	return false, nil
}

// DecoderInjector is used by the ControllerManager to inject decoder into webhook handlers.
type DecoderInjector interface {
	InjectDecoder(*admission.Decoder) error
}

// DecoderInto will set decoder on i and return the result if it implements Decoder.  Returns
// false if i does not implement Decoder.
func DecoderInto(decoder *admission.Decoder, i interface{}) (bool, error) {
	if s, ok := i.(DecoderInjector); ok {
		return true, s.InjectDecoder(decoder)
	}
	return false, nil
}
