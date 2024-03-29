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

package main

import (
	"fmt"

	"github.com/drycc-addons/service-catalog/pkg/svcat"
	servicecatalog "github.com/drycc-addons/service-catalog/pkg/svcat/service-catalog"
	"k8s.io/klog/v2"
)

func main() {
	klog.InitFlags(nil)
	a, _ := svcat.NewApp(nil, nil, "")
	brokers, _ := a.RetrieveBrokers(servicecatalog.ScopeOptions{Scope: servicecatalog.AllScope})
	for _, b := range brokers {
		fmt.Println(b.GetName())
	}
}
