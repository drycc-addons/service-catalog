/*
Copyright 2017 The Kubernetes Authors.

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
	sc "github.com/drycc-addons/service-catalog/pkg/apis/servicecatalog/v1beta1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"regexp"
)

var hexademicalStringRegexp = regexp.MustCompile("^[[:xdigit:]]*$")

func stringIsHexadecimal(s string) bool {
	return hexademicalStringRegexp.MatchString(s)
}

func validateParametersFromSource(parametersFrom []sc.ParametersFromSource, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	for _, paramsFrom := range parametersFrom {
		if paramsFrom.SecretKeyRef != nil {
			if paramsFrom.SecretKeyRef.Name == "" {
				allErrs = append(allErrs, field.Required(fldPath.Child("parametersFrom.secretKeyRef.name"), "name is required"))
			}
			if paramsFrom.SecretKeyRef.Key == "" {
				allErrs = append(allErrs, field.Required(fldPath.Child("parametersFrom.secretKeyRef.key"), "key is required"))
			}
		} else {
			allErrs = append(allErrs, field.Required(fldPath.Child("parametersFrom"), "source must not be empty if present"))
		}
	}

	return allErrs
}
