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

package command

import (
	"io"

	"github.com/drycc-addons/service-catalog/pkg/svcat"
	"github.com/spf13/viper"
)

// Context is ambient data necessary to run any svcat command.
type Context struct {
	// Output should be used instead of directly writing to stdout/stderr, to enable unit testing.
	Output io.Writer

	// svcat application, the library behind the cli
	App *svcat.App

	// Viper configuration
	Viper *viper.Viper
}
