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

package validation

import (
	"context"
	"net/http"

	sc "github.com/drycc-addons/service-catalog/pkg/apis/servicecatalog/v1beta1"
	"github.com/drycc-addons/service-catalog/pkg/webhook/inject"
	"github.com/drycc-addons/service-catalog/pkg/webhookutil"
	admissionTypes "k8s.io/api/admission/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// Validator is used to implement new validation logic
type Validator interface {
	Validate(context.Context, admission.Request, *sc.ServiceBroker, *webhookutil.TracedLogger) *webhookutil.WebhookError
}

// SpecValidationHandler handles ServiceBroker validation
type SpecValidationHandler struct {
	decoder admission.Decoder

	CreateValidators []Validator
	UpdateValidators []Validator
}

// NewSpecValidationHandler creates new SpecValidationHandler and initializes validators list
func NewSpecValidationHandler() *SpecValidationHandler {
	return &SpecValidationHandler{
		CreateValidators: []Validator{&StaticCreate{}, &AccessToBroker{}},
		UpdateValidators: []Validator{&StaticUpdate{}, &AccessToBroker{}},
	}
}

// Handle handles admission requests.
func (h *SpecValidationHandler) Handle(ctx context.Context, req admission.Request) admission.Response {
	traced := webhookutil.NewTracedLogger(req.UID)
	traced.Infof("Start handling validation operation: %s for %s", req.Operation, req.Kind.Kind)

	sb := &sc.ServiceBroker{}
	if err := webhookutil.MatchKinds(sb, req.Kind); err != nil {
		traced.Errorf("Error matching kinds: %v", err)
		return admission.Errored(http.StatusBadRequest, err)
	}

	if err := h.decoder.Decode(req, sb); err != nil {
		traced.Errorf("Could not decode request object: %v", err)
		return admission.Errored(http.StatusBadRequest, err)
	}

	traced.Infof("start validation process for %s: %s/%s", sb.Kind, sb.Namespace, sb.Name)

	var err *webhookutil.WebhookError

	switch req.Operation {
	case admissionTypes.Create:
		for _, v := range h.CreateValidators {
			err = v.Validate(ctx, req, sb, traced)
			if err != nil {
				break
			}
		}
	case admissionTypes.Update:
		for _, v := range h.UpdateValidators {
			err = v.Validate(ctx, req, sb, traced)
			if err != nil {
				break
			}
		}
	default:
		traced.Infof("ServiceBroker validation wehbook does not support action %q", req.Operation)
		return admission.Allowed("action not taken")
	}

	if err != nil {
		switch err.Code() {
		case http.StatusForbidden:
			return admission.Denied(err.Error())
		default:
			return admission.Errored(err.Code(), err)
		}
	}

	traced.Infof("Completed successfully validation operation: %s for %s: %q", req.Operation, req.Kind.Kind, req.Name)
	return admission.Allowed("ServiceBroker validation successful")
}

// InjectDecoder injects the decoder into the handlers
func (h *SpecValidationHandler) InjectDecoder(d admission.Decoder) error {
	h.decoder = d

	for _, v := range h.CreateValidators {
		_, err := inject.DecoderInto(d, v)
		if err != nil {
			return err
		}
	}
	for _, v := range h.UpdateValidators {
		_, err := inject.DecoderInto(d, v)
		if err != nil {
			return err
		}
	}

	return nil
}

// InjectClient injects the client into the handlers
func (h *SpecValidationHandler) InjectClient(c client.Client) error {
	for _, v := range h.CreateValidators {
		_, err := inject.ClientInto(c, v)
		if err != nil {
			return err
		}
	}
	for _, v := range h.UpdateValidators {
		_, err := inject.ClientInto(c, v)
		if err != nil {
			return err
		}
	}

	return nil
}
