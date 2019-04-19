package lib

import (
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
	"gopkg.in/yaml.v2"
)

func NewRuntime(c []byte) (lambda.Handler, func() error, error) {
	var transactionChan chan types.Transaction
	var closeFn func() error

	conf := config.New()

	if err := yaml.Unmarshal(c, &conf); err != nil {
		return nil, nil, fmt.Errorf("Configuration file read error: %v", err)
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
		return nil, nil, fmt.Errorf("Failed to create resource: %v", err)
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
		outputLayer = &InterceptingOutput{Output: outputLayer}
	}
	if err == nil {
		err = pipelineLayer.Consume(transactionChan)
	}
	if err == nil {
		err = outputLayer.Consume(pipelineLayer.TransactionChan())
	}
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to create resource: %v", err)
	}

	closeFn = func() error {
		exitTimeout := time.Second * 30
		timesOut := time.Now().Add(exitTimeout)
		pipelineLayer.CloseAsync()
		outputLayer.CloseAsync()
		if err = outputLayer.WaitForClose(exitTimeout); err != nil {
			return err
		}

		manager.CloseAsync()
		if err = manager.WaitForClose(time.Until(timesOut)); err != nil {
			logger.Warnf(
				"Service failed to close cleanly within allocated time: %v."+
					" Exiting forcefully and dumping stack trace to stderr.\n", err,
			)
			pprof.Lookup("goroutine").WriteTo(os.Stderr, 1)
			return err
		}

		trac.Close()

		if sCloseErr := stats.Close(); sCloseErr != nil {
			return fmt.Errorf("Failed to cleanly close metrics aggregator: %v", sCloseErr)
		}
		return nil
	}
	handler := &Handler{
		Transactions: transactionChan,
	}
	return lambda.NewHandler(handler.Handle), closeFn, nil
}
