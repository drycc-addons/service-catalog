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

package server

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net/http"
	"time"

	"github.com/drycc-addons/service-catalog/contrib/pkg/broker/controller"
	"github.com/drycc-addons/service-catalog/contrib/pkg/brokerapi"
	"github.com/drycc-addons/service-catalog/pkg/util"
	"k8s.io/klog/v2"

	"github.com/gorilla/mux"
)

type server struct {
	controller controller.Controller
}

// ErrorWithHTTPStatus is an error that also defines the HTTP status code
// that should be returned to the client making the request
type ErrorWithHTTPStatus struct {
	err        string
	httpStatus int
}

// NewErrorWithHTTPStatus creates a new ErrorWithHTTPStatus with the given
// error message and HTTP status code
func NewErrorWithHTTPStatus(err string, httpStatus int) ErrorWithHTTPStatus {
	return ErrorWithHTTPStatus{err, httpStatus}
}

func (e ErrorWithHTTPStatus) Error() string {
	return e.err
}

// HTTPStatus returns the HTTP status code that should be returned to the client
func (e ErrorWithHTTPStatus) HTTPStatus() int {
	return e.httpStatus
}

// CreateHandler creates Broker HTTP handler based on an implementation
// of a controller.Controller interface.
func createHandler(c controller.Controller) http.Handler {
	s := server{
		controller: c,
	}

	var router = mux.NewRouter()

	router.HandleFunc("/v2/catalog", s.catalog).Methods("GET")
	router.HandleFunc("/v2/service_instances/{instance_id}/last_operation", s.getServiceInstanceLastOperation).Methods("GET")
	router.HandleFunc("/v2/service_instances/{instance_id}", s.createServiceInstance).Methods("PUT")
	router.HandleFunc("/v2/service_instances/{instance_id}", s.updateServiceInstance).Methods("PATCH")
	router.HandleFunc("/v2/service_instances/{instance_id}", s.removeServiceInstance).Methods("DELETE")
	router.HandleFunc("/v2/service_instances/{instance_id}/service_bindings/{binding_id}", s.bind).Methods("PUT")
	router.HandleFunc("/v2/service_instances/{instance_id}/service_bindings/{binding_id}", s.unBind).Methods("DELETE")

	return router
}

// Run creates the HTTP handler based on an implementation of a
// controller.Controller interface, and begins to listen on the specified address.
func Run(ctx context.Context, addr string, c controller.Controller) error {
	listenAndServe := func(srv *http.Server) error {
		return srv.ListenAndServe()
	}
	return run(ctx, addr, listenAndServe, c)
}

// RunTLS creates the HTTPS handler based on an implementation of a
// controller.Controller interface, and begins to listen on the specified address.
func RunTLS(ctx context.Context, addr string, cert string, key string, c controller.Controller) error {
	var decodedCert, decodedKey []byte
	var tlsCert tls.Certificate
	var err error
	decodedCert, err = base64.StdEncoding.DecodeString(cert)
	if err != nil {
		return err
	}
	decodedKey, err = base64.StdEncoding.DecodeString(key)
	if err != nil {
		return err
	}
	tlsCert, err = tls.X509KeyPair(decodedCert, decodedKey)
	if err != nil {
		return err
	}
	listenAndServe := func(srv *http.Server) error {
		srv.TLSConfig = new(tls.Config)
		srv.TLSConfig.Certificates = []tls.Certificate{tlsCert}
		return srv.ListenAndServeTLS("", "")
	}
	return run(ctx, addr, listenAndServe, c)
}

func run(ctx context.Context, addr string, listenAndServe func(srv *http.Server) error, c controller.Controller) error {
	klog.Infof("Starting server on %s\n", addr)
	srv := &http.Server{
		Addr:    addr,
		Handler: createHandler(c),
	}
	go func() {
		<-ctx.Done()
		c, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if srv.Shutdown(c) != nil {
			srv.Close()
		}
	}()
	return listenAndServe(srv)
}

func (s *server) catalog(w http.ResponseWriter, r *http.Request) {
	klog.Infof("Get Service Broker Catalog...")

	if result, err := s.controller.Catalog(); err == nil {
		util.WriteResponse(w, http.StatusOK, result)
	} else {
		util.WriteErrorResponse(w, getHTTPStatus(err), err)
	}
}

func (s *server) getServiceInstanceLastOperation(w http.ResponseWriter, r *http.Request) {
	instanceID := mux.Vars(r)["instance_id"]
	q := r.URL.Query()
	serviceID := q.Get("service_id")
	planID := q.Get("plan_id")
	operation := q.Get("operation")
	klog.Infof("GetServiceInstance ... %s\n", instanceID)

	if result, err := s.controller.GetServiceInstanceLastOperation(instanceID, serviceID, planID, operation); err == nil {
		util.WriteResponse(w, http.StatusOK, result)
	} else {
		util.WriteErrorResponse(w, getHTTPStatus(err), err)
	}
}

func (s *server) createServiceInstance(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["instance_id"]
	klog.Infof("CreateServiceInstance %s...\n", id)

	var req brokerapi.CreateServiceInstanceRequest
	if err := util.BodyToObject(r, &req); err != nil {
		klog.Errorf("error unmarshalling: %v", err)
		util.WriteErrorResponse(w, getHTTPStatus(err), err)
		return
	}

	// TODO: Check if parameters are required, if not, this thing below is ok to leave in,
	// if they are ,they should be checked. Because if no parameters are passed in, this will
	// fail because req.Parameters is nil.
	if req.Parameters == nil {
		req.Parameters = make(map[string]interface{})
	}

	if result, err := s.controller.CreateServiceInstance(id, &req); err == nil {
		if result.Operation == "" {
			util.WriteResponse(w, http.StatusCreated, result) // TODO: return StatusOK if instance already exists
		} else {
			util.WriteResponse(w, http.StatusAccepted, result)
		}
	} else {
		util.WriteErrorResponse(w, getHTTPStatus(err), err)
	}
}

func (s *server) updateServiceInstance(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["instance_id"]
	klog.Infof("UpdateServiceInstance %s...\n", id)

	var req brokerapi.UpdateServiceInstanceRequest
	if err := util.BodyToObject(r, &req); err != nil {
		klog.Errorf("error unmarshalling: %v", err)
		util.WriteErrorResponse(w, getHTTPStatus(err), err)
		return
	}

	// TODO: Check if parameters are required, if not, this thing below is ok to leave in,
	// if they are ,they should be checked. Because if no parameters are passed in, this will
	// fail because req.Parameters is nil.
	if req.Parameters == nil {
		req.Parameters = make(map[string]interface{})
	}

	if result, err := s.controller.UpdateServiceInstance(id, &req); err == nil {
		if result.Operation == "" {
			util.WriteResponse(w, http.StatusOK, result)
		} else {
			util.WriteResponse(w, http.StatusAccepted, result)
		}
	} else {
		util.WriteErrorResponse(w, getHTTPStatus(err), err)
	}
}

func (s *server) removeServiceInstance(w http.ResponseWriter, r *http.Request) {
	instanceID := mux.Vars(r)["instance_id"]
	q := r.URL.Query()
	serviceID := q.Get("service_id")
	planID := q.Get("plan_id")
	acceptsIncomplete := q.Get("accepts_incomplete") == "true"
	klog.Infof("RemoveServiceInstance %s...\n", instanceID)

	if result, err := s.controller.RemoveServiceInstance(instanceID, serviceID, planID, acceptsIncomplete); err == nil {
		if result.Operation == "" {
			util.WriteResponse(w, http.StatusOK, result)
		} else {
			util.WriteResponse(w, http.StatusAccepted, result)
		}
	} else {
		util.WriteErrorResponse(w, getHTTPStatus(err), err)
	}
}

func getHTTPStatus(err error) int {
	if err, ok := err.(ErrorWithHTTPStatus); ok {
		return err.HTTPStatus()
	}
	return http.StatusBadRequest
}

func (s *server) bind(w http.ResponseWriter, r *http.Request) {
	bindingID := mux.Vars(r)["binding_id"]
	instanceID := mux.Vars(r)["instance_id"]

	klog.Infof("Bind binding_id=%s, instance_id=%s\n", bindingID, instanceID)

	var req brokerapi.BindingRequest

	if err := util.BodyToObject(r, &req); err != nil {
		klog.Errorf("Failed to unmarshall request: %v", err)
		util.WriteErrorResponse(w, getHTTPStatus(err), err)
		return
	}

	// TODO: Check if parameters are required, if not, this thing below is ok to leave in,
	// if they are ,they should be checked. Because if no parameters are passed in, this will
	// fail because req.Parameters is nil.
	if req.Parameters == nil {
		req.Parameters = make(map[string]interface{})
	}

	// Pass in the instanceId to the template.
	req.Parameters["instanceId"] = instanceID

	if result, err := s.controller.Bind(instanceID, bindingID, &req); err == nil {
		util.WriteResponse(w, http.StatusOK, result)
	} else {
		util.WriteErrorResponse(w, getHTTPStatus(err), err)
	}
}

func (s *server) unBind(w http.ResponseWriter, r *http.Request) {
	instanceID := mux.Vars(r)["instance_id"]
	bindingID := mux.Vars(r)["binding_id"]
	q := r.URL.Query()
	serviceID := q.Get("service_id")
	planID := q.Get("plan_id")
	klog.Infof("UnBind: Service instance guid: %s:%s", bindingID, instanceID)

	if err := s.controller.UnBind(instanceID, bindingID, serviceID, planID); err == nil {
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, "{}") //id)
	} else {
		util.WriteErrorResponse(w, getHTTPStatus(err), err)
	}
}
