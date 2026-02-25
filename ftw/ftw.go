/*
Copyright 2026 Shane Utt.

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
	"context"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/coreruleset/ftw-tests-schema/v2/types"
	"github.com/coreruleset/go-ftw/v2/config"
	"github.com/coreruleset/go-ftw/v2/output"
	"github.com/coreruleset/go-ftw/v2/runner"
	"github.com/coreruleset/go-ftw/v2/test"
	"github.com/rs/zerolog"
	"github.com/spf13/pflag"
	v1 "k8s.io/api/core/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
)

type FTWRunner struct {
	Tests       []*test.FTWTest
	Config      *config.RunnerConfig
	Client      *kubernetes.Clientset
	GatewayName string
	tmpLogFile  string
	deferFunc   []func() error
}

func main() {
	configFlags := genericclioptions.NewConfigFlags(true)
	configFlags.AddFlags(pflag.CommandLine)
	ftwConfigurationFile := pflag.String("ftw-config", "", "Define the full path for ftw.yml configuration file")
	ftwRulesDirectory := pflag.String("ftw-rules-dir", "", "Define the full path for ftw rules directory")
	ignoreTestLoadError := pflag.Bool("ignore-test-load-error", false, "If true it will not error and exit in case some test specification have some error")
	hostname := pflag.String("hostname", "", "Hostname header to be used on FTW tests. If empty, just the IP will be used")
	gatewayName := pflag.String("gateway-name", "", "Name of the Gateway that will be tested")
	pflag.Parse()

	if *ftwConfigurationFile == "" || *ftwRulesDirectory == "" || *gatewayName == "" {
		fatal(fmt.Errorf("gateway-name, ftw-config and ftw-rules-dir are mandatory arguments"))
	}

	_, err := os.Stat(*ftwConfigurationFile)
	if err != nil {
		fatal(fmt.Errorf("error loading ftw config: %w", err))
	}

	k8sConfig, err := configFlags.ToRESTConfig()
	if err != nil {
		fatal(fmt.Errorf("error loading k8s config: %w", err))
	}

	client := kubernetes.NewForConfigOrDie(k8sConfig)
	ftwcfg, err := config.NewConfigFromFile(*ftwConfigurationFile)
	if err != nil {
		fatal(fmt.Errorf("error parsing ftw configuration: %w", err))
	}
	ftwcfg.TestOverride.Overrides.DestAddr = new("172.18.255.129")
	ftwcfg.TestOverride.Overrides.Port = new(80)

	if *hostname != "" {
		ftwcfg.TestOverride.Overrides.VirtualHostMode = new(true)
		ftwcfg.TestOverride.Overrides.OrderedHeaders = []types.HeaderTuple{
			{
				Name:  "Host",
				Value: *hostname,
			},
		}
	}

	// FTW has a global logging directive, needs to be changed soon
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	runnerConfig := config.NewRunnerConfiguration(ftwcfg)
	runnerConfig.ShowTime = true

	fsDir, err := os.OpenRoot(*ftwRulesDirectory)
	if err != nil {
		fatal(err)
	}

	var tests []*test.FTWTest
	err = doublestar.GlobWalk(fsDir.FS(), "**/*.yaml", func(path string, d os.DirEntry) error {
		yaml, err := fs.ReadFile(fsDir.FS(), path)
		if err != nil {
			return fmt.Errorf("error reading rules file %s: %w", path, err)
		}
		ftwt, err := test.GetTestFromYaml(yaml, path)
		if err != nil {
			if *ignoreTestLoadError {
				slog.Error("error loading test from file, skipping", "file", path, "error", err)
				return nil
			}
			return fmt.Errorf("error loading test from file %s: %w", path, err)
		}

		tests = append(tests, ftwt)
		return nil
	})
	if err != nil {
		fatal(err)
	}
	if len(tests) == 0 {
		fatal(fmt.Errorf("no tests were found"))
	}
	ftwrunner := &FTWRunner{
		Client:      client,
		Config:      runnerConfig,
		Tests:       tests,
		GatewayName: *gatewayName,
		deferFunc:   make([]func() error, 0),
	}
	ctx := context.Background()
	if err := ftwrunner.Init(ctx); err != nil {
		panic(err)
	}
	if err := ftwrunner.Run(ctx); err != nil {
		panic(err)
	}

}

func fatal(err error) {
	slog.Error("error executing ftw", "error", err)
	os.Exit(1)
}

// Init initializes FTW before running it. It must at least start the logger for the
// Gateway and fetch the Gateway IP and port before it starts (we assume port as 80 for now)
func (f *FTWRunner) Init(ctx context.Context) error {
	tmpFile, err := os.CreateTemp("", "")
	if err != nil {
		return err
	}
	stream, err := f.Client.CoreV1().Pods("ftw-test").GetLogs("coraza-gateway-istio-54cf6f595f-46qrz", &v1.PodLogOptions{
		Follow: true,
	}).Stream(ctx)
	if err != nil {
		return err
	}
	f.deferFunc = append(f.deferFunc, stream.Close)

	go func() {
		io.Copy(tmpFile, stream)
	}()

	f.Config.LogFilePath = tmpFile.Name()
	return nil

}

func (f *FTWRunner) Run(ctx context.Context) error {
	res, err := runner.Run(f.Config, f.Tests, output.NewOutput("quiet", os.Stdout))
	if err != nil {
		return err
	}

	if len(res.Stats.Failed) > 0 {
		slog.Error("failed tests", "failed", res.Stats.Failed)
		return fmt.Errorf("some tests failed")
	}
	return nil
}
