// Copyright (c) 2019 Ashley Jeffs
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package main

import (
	"context"
	"fmt"
	"os"
	"runtime/pprof"
	"time"

	"github.com/Jeffail/benthos/lib/config"
	"github.com/Jeffail/benthos/lib/log"
	"github.com/Jeffail/benthos/lib/manager"
	"github.com/Jeffail/benthos/lib/metrics"
	"github.com/Jeffail/benthos/lib/output"
	"github.com/Jeffail/benthos/lib/pipeline"
	"github.com/Jeffail/benthos/lib/tracer"
	"github.com/Jeffail/benthos/lib/types"
	"github.com/aws/aws-lambda-go/lambda"
)

//------------------------------------------------------------------------------

var transactionChan chan types.Transaction
var closeFn func()

func init() {
	// A list of default config paths to check for if not explicitly defined
	defaultPaths := []string{
		"/benthos.yaml",
		"/etc/benthos/config.yaml",
		"/etc/benthos.yaml",
	}

	conf := config.New()

	// Iterate default config paths
	for _, path := range defaultPaths {
		if _, err := os.Stat(path); err == nil {
			fmt.Fprintf(os.Stderr, "Config file not specified, reading from %v\n", path)

			if _, err = config.Read(path, true, &conf); err != nil {
				fmt.Fprintf(os.Stderr, "Configuration file read error: %v\n", err)
				os.Exit(1)
			}
			break
		}
	}

	// Logging and stats aggregation.
	logger := log.New(os.Stdout, conf.Logger)

	// Create our metrics type.
	stats, err := metrics.New(conf.Metrics, metrics.OptSetLogger(logger))
	for err != nil {
		logger.Errorf("Failed to connect metrics aggregator: %v\n", err)
		stats = metrics.Noop()
	}

	// Create our tracer type.
	var trac tracer.Type
	if trac, err = tracer.New(conf.Tracer); err != nil {
		logger.Errorf("Failed to initialise tracer: %v\n", err)
		trac = tracer.Noop()
	}

	// Create resource manager.
	manager, err := manager.New(conf.Manager, types.NoopMgr(), logger, stats)
	if err != nil {
		logger.Errorf("Failed to create resource: %v\n", err)
		os.Exit(1)
	}

	// Create pipeline and output layers.
	var pipelineLayer types.Pipeline
	var outputLayer types.Output

	transactionChan = make(chan types.Transaction)

	pipelineLayer, err = pipeline.New(
		conf.Pipeline, manager,
		logger.NewModule(".pipeline"), metrics.Namespaced(stats, "pipeline"),
	)
	if err == nil {
		outputLayer, err = output.New(
			conf.Output, manager,
			logger.NewModule(".output"), metrics.Namespaced(stats, "output"),
		)
	}
	if err == nil {
		err = pipelineLayer.Consume(transactionChan)
	}
	if err == nil {
		err = outputLayer.Consume(pipelineLayer.TransactionChan())
	}
	if err != nil {
		logger.Errorf("Failed to create resource: %v\n", err)
		os.Exit(1)
	}

	closeFn = func() {
		exitTimeout := time.Second * 30
		timesOut := time.Now().Add(exitTimeout)
		pipelineLayer.CloseAsync()
		outputLayer.CloseAsync()
		if err = outputLayer.WaitForClose(exitTimeout); err != nil {
			os.Exit(1)
		}

		manager.CloseAsync()
		if err = manager.WaitForClose(time.Until(timesOut)); err != nil {
			logger.Warnf(
				"Service failed to close cleanly within allocated time: %v."+
					" Exiting forcefully and dumping stack trace to stderr.\n", err,
			)
			pprof.Lookup("goroutine").WriteTo(os.Stderr, 1)
			os.Exit(1)
		}

		trac.Close()

		if sCloseErr := stats.Close(); sCloseErr != nil {
			logger.Errorf("Failed to cleanly close metrics aggregator: %v\n", sCloseErr)
		}
	}
}

func HandleFilterRequest(ctx context.Context, obj interface{}) (interface{}, error) {
	// TODO
	return nil, nil
}

func main() {
	lambda.Start(HandleFilterRequest)
}

//------------------------------------------------------------------------------
