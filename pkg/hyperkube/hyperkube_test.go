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

package hyperkube

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/drycc-addons/service-catalog/cmd/webhook/server"
	"github.com/drycc-addons/service-catalog/test/util"
)

type result struct {
	err    error
	output string
}

func testServer(n string) *Server {
	return &Server{
		SimpleUsage: n,
		Long:        fmt.Sprintf("A simple server named %s", n),
		Run: func(s *Server, args []string, stopCh <-chan struct{}) error {
			s.hk.Printf("%s Run\n", s.Name())
			return nil
		},
	}
}

func testServerError(n string) *Server {
	return &Server{
		SimpleUsage: n,
		Long:        fmt.Sprintf("A simple server named %s that returns an error", n),
		Run: func(s *Server, args []string, stopCh <-chan struct{}) error {
			s.hk.Printf("%s Run\n", s.Name())
			return errors.New("server returning error")
		},
	}
}
func testStopChRespectingServer(n string) *Server {
	return &Server{
		SimpleUsage: n,
		Long:        fmt.Sprintf("A simple server named %s", n),
		Run: func(s *Server, args []string, stopCh <-chan struct{}) error {
			s.hk.Printf("%s Run\n", s.Name())
			<-stopCh
			return nil
		},
		RespectsStopCh: true,
	}
}
func testStopChIgnoringServer(n string) *Server {
	return &Server{
		SimpleUsage: n,
		Long:        fmt.Sprintf("A simple server named %s", n),
		Run: func(s *Server, args []string, stopCh <-chan struct{}) error {
			<-wait.NeverStop // this leaks obviously, but we don't care about one go routine more or less in test
			return nil
		},
		RespectsStopCh: false,
	}
}
func testStopChRespectingServerWithError(n string) *Server {
	return &Server{
		SimpleUsage: n,
		Long:        fmt.Sprintf("A simple server named %s", n),
		Run: func(s *Server, args []string, stopCh <-chan struct{}) error {
			s.hk.Printf("%s Run\n", s.Name())
			<-stopCh
			return errors.New("server returning error")
		},
		RespectsStopCh: true,
	}
}

const defaultCobraMessage = "default message from cobra command"
const defaultCobraSubMessage = "default sub-message from cobra command"
const cobraMessageDesc = "message to print"
const cobraSubMessageDesc = "sub-message to print"

func testCobraCommand(n string) *Server {

	var cobraServer *Server
	var msg string
	cmd := &cobra.Command{
		Use:   n,
		Long:  n,
		Short: n,
		Run: func(cmd *cobra.Command, args []string) {
			cobraServer.hk.Printf("msg: %s\n", msg)
		},
	}
	cmd.PersistentFlags().StringVar(&msg, "msg", defaultCobraMessage, cobraMessageDesc)

	var subMsg string
	subCmdName := "subcommand"
	subCmd := &cobra.Command{
		Use:   subCmdName,
		Long:  subCmdName,
		Short: subCmdName,
		Run: func(cmd *cobra.Command, args []string) {
			cobraServer.hk.Printf("submsg: %s", subMsg)
		},
	}
	subCmd.PersistentFlags().StringVar(&subMsg, "submsg", defaultCobraSubMessage, cobraSubMessageDesc)

	cmd.AddCommand(subCmd)

	localFlags := cmd.LocalFlags()
	localFlags.SetInterspersed(false)
	s := &Server{
		SimpleUsage: n,
		Long:        fmt.Sprintf("A server named %s which uses a cobra command", n),
		Run: func(s *Server, args []string, stopCh <-chan struct{}) error {
			cobraServer = s
			cmd.SetOutput(s.hk.Out())
			cmd.SetArgs(args)
			return cmd.Execute()
		},
		flags: localFlags,
	}

	return s
}
func runFull(t *testing.T, args string, stopCh <-chan struct{}) *result {
	buf := new(bytes.Buffer)
	hk := HyperKube{
		Name: "hyperkube",
		Long: "hyperkube is an all-in-one server binary.",
	}
	hk.SetOut(buf)

	hk.AddServer(testServer("test1"))
	hk.AddServer(testServer("test2"))
	hk.AddServer(testServer("test3"))
	hk.AddServer(testServerError("test-error"))
	hk.AddServer(testStopChIgnoringServer("test-stop-ch-ignoring"))
	hk.AddServer(testStopChRespectingServer("test-stop-ch-respecting"))
	hk.AddServer(testStopChRespectingServerWithError("test-error-stop-ch-respecting"))
	hk.AddServer(testCobraCommand("test-cobra-command"))

	a := strings.Split(args, " ")
	t.Logf("Running full with args: %q", a)
	err := hk.Run(a, stopCh)

	r := &result{err, buf.String()}
	t.Logf("Result err: %v, output: %q", r.err, r.output)

	return r
}

func TestRun(t *testing.T) {
	x := runFull(t, "hyperkube test1", wait.NeverStop)
	util.AssertContains(t, x.output, "test1 Run")
	util.AssertNoError(t, x.err)
}

func TestLinkRun(t *testing.T) {
	x := runFull(t, "test1", wait.NeverStop)
	util.AssertContains(t, x.output, "test1 Run")
	util.AssertNoError(t, x.err)
}

func TestTopNoArgs(t *testing.T) {
	x := runFull(t, "hyperkube", wait.NeverStop)
	util.AssertEqualError(t, x.err, "no server specified")
}

func TestBadServer(t *testing.T) {
	x := runFull(t, "hyperkube bad-server", wait.NeverStop)
	util.AssertEqualError(t, x.err, "Server not found: bad-server")
	util.AssertContains(t, x.output, "Usage")
}

func TestTopHelp(t *testing.T) {
	x := runFull(t, "hyperkube --help", wait.NeverStop)
	util.AssertNoError(t, x.err)
	util.AssertContains(t, x.output, "all-in-one")
	util.AssertContains(t, x.output, "A simple server named test1")
}

func TestTopFlags(t *testing.T) {
	x := runFull(t, "hyperkube --help test1", wait.NeverStop)
	util.AssertNoError(t, x.err)
	util.AssertContains(t, x.output, "all-in-one")
	util.AssertContains(t, x.output, "A simple server named test1")
	util.AssertNotContains(t, x.output, "test1 Run")
}

func TestTopFlagsBad(t *testing.T) {
	x := runFull(t, "hyperkube --bad-flag", wait.NeverStop)
	util.AssertEqualError(t, x.err, "unknown flag: --bad-flag")
	util.AssertContains(t, x.output, "all-in-one")
	util.AssertContains(t, x.output, "A simple server named test1")
}

func TestServerHelp(t *testing.T) {
	x := runFull(t, "hyperkube test1 --help", wait.NeverStop)
	util.AssertNoError(t, x.err)
	util.AssertContains(t, x.output, "A simple server named test1")
	util.AssertContains(t, x.output, "-h, --help")
	util.AssertContains(t, x.output, "help for hyperkube")
	util.AssertNotContains(t, x.output, "test1 Run")
}

func TestServerFlagsBad(t *testing.T) {
	x := runFull(t, "hyperkube test1 --bad-flag", wait.NeverStop)
	util.AssertEqualError(t, x.err, "unknown flag: --bad-flag")
	util.AssertContains(t, x.output, "A simple server named test1")
	util.AssertContains(t, x.output, "-h, --help")
	util.AssertContains(t, x.output, "help for hyperkube")
	util.AssertNotContains(t, x.output, "test1 Run")
}

func TestServerError(t *testing.T) {
	x := runFull(t, "hyperkube test-error", wait.NeverStop)
	util.AssertContains(t, x.output, "test-error Run")
	util.AssertEqualError(t, x.err, "server returning error")
}

func TestStopChIgnoringServer(t *testing.T) {
	stopCh := make(chan struct{})
	returnedCh := make(chan struct{})
	var x *result
	go func() {
		defer close(returnedCh)
		x = runFull(t, "hyperkube test-stop-ch-ignoring", stopCh)
	}()
	close(stopCh)
	select {
	case <-time.After(wait.ForeverTestTimeout):
		t.Fatalf("%q never returned after stopCh was closed", "hyperkube test-stop-ch-ignoring")
	case <-returnedCh:
	}
	// we cannot be sure that the server had a chance to output anything
	// util.Assert.Contains(t, x.output, "test-error-stop-ch-ignoring Run")
	util.AssertEqualError(t, x.err, "interrupted")
}

func TestStopChRespectingServer(t *testing.T) {
	stopCh := make(chan struct{})
	returnedCh := make(chan struct{})
	var x *result
	go func() {
		defer close(returnedCh)
		x = runFull(t, "hyperkube test-stop-ch-respecting", stopCh)
	}()
	close(stopCh)
	select {
	case <-time.After(wait.ForeverTestTimeout):
		t.Fatalf("%q never returned after stopCh was closed", "hyperkube test-stop-ch-respecting")
	case <-returnedCh:
	}
	util.AssertContains(t, x.output, "test-stop-ch-respecting Run")
	util.AssertNil(t, x.err)
}

func TestStopChRespectingServerWithError(t *testing.T) {
	stopCh := make(chan struct{})
	returnedCh := make(chan struct{})
	var x *result
	go func() {
		defer close(returnedCh)
		x = runFull(t, "hyperkube test-error-stop-ch-respecting", stopCh)
	}()
	close(stopCh)
	select {
	case <-time.After(wait.ForeverTestTimeout):
		t.Fatalf("%q never returned after stopCh was closed", "hyperkube test-error-stop-ch-respecting")
	case <-returnedCh:
	}
	util.AssertContains(t, x.output, "test-error-stop-ch-respecting Run")
	util.AssertEqualError(t, x.err, "server returning error")
}

func TestCobraCommandHelp(t *testing.T) {
	x := runFull(t, "hyperkube test-cobra-command --help", wait.NeverStop)
	util.AssertNoError(t, x.err)
	util.AssertContains(t, x.output, "A server named test-cobra-command which uses a cobra command")
	util.AssertContains(t, x.output, cobraMessageDesc)
}
func TestCobraCommandDefaultMessage(t *testing.T) {
	x := runFull(t, "hyperkube test-cobra-command", wait.NeverStop)
	util.AssertContains(t, x.output, fmt.Sprintf("msg: %s", defaultCobraMessage))
}
func TestCobraCommandMessage(t *testing.T) {
	x := runFull(t, "hyperkube test-cobra-command --msg foobar", wait.NeverStop)
	util.AssertContains(t, x.output, "msg: foobar")
}

func TestCobraSubCommandHelp(t *testing.T) {
	x := runFull(t, "hyperkube test-cobra-command subcommand --help", wait.NeverStop)
	util.AssertNoError(t, x.err)
	util.AssertContains(t, x.output, cobraSubMessageDesc)
}
func TestCobraSubCommandDefaultMessage(t *testing.T) {
	x := runFull(t, "hyperkube test-cobra-command subcommand", wait.NeverStop)
	util.AssertContains(t, x.output, fmt.Sprintf("submsg: %s", defaultCobraSubMessage))
}
func TestCobraSubCommandMessage(t *testing.T) {
	x := runFull(t, "hyperkube test-cobra-command subcommand --submsg foobar", wait.NeverStop)
	util.AssertContains(t, x.output, "submsg: foobar")
}

func testRunServer(n string) *Server {
	s := server.NewWebhookServerOptions()
	return &Server{
		PrimaryName:     "testserver",
		AlternativeName: "service-catalog-testserver",
		SimpleUsage:     "testserver",
		Long:            fmt.Sprintf("A simple server named %s", n),
		Run: func(_ *Server, args []string, stopCh <-chan struct{}) error {
			return server.RunServer(s, stopCh)
		},
		RespectsStopCh: true,
	}
}

func TestStopChannelWithRespectsStopCh(t *testing.T) {
	expect := fmt.Errorf("%v", "stop channel was not set when starting the api server")
	hk := HyperKube{
		Name: "hyperkube",
		Long: "hyperkube is an all-in-one server binary.",
	}

	hk.AddServer(testRunServer("test1"))
	args := []string{"testserver"}

	defer func() {
		if value := recover(); value != nil {
			util.AssertEqualError(t, expect, value.(string))
		}
	}()

	hk.Run(args, nil)
}
