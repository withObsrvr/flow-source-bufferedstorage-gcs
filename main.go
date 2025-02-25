package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/pkg/errors"
	"github.com/stellar/go/xdr"
	cdp "github.com/withObsrvr/stellar-cdp"
	datastore "github.com/withObsrvr/stellar-datastore"
	ledgerbackend "github.com/withObsrvr/stellar-ledgerbackend"

	// Import the core plugin API definitions. Adjust the import path as needed.

	"github.com/withObsrvr/pluginapi"
)

// BufferedStorageConfig holds configuration values for the source.
type BufferedStorageConfig struct {
	BucketName        string
	BufferSize        uint32
	NumWorkers        uint32
	RetryLimit        uint32
	RetryWait         uint32
	Network           string
	StartLedger       uint32
	EndLedger         uint32
	LedgersPerFile    uint32
	FilesPerPartition uint32
}

// BufferedStorageSourceAdapter implements pluginapi.Source.
type BufferedStorageSourceAdapter struct {
	config     BufferedStorageConfig
	processors []pluginapi.Processor
}

// Name returns the plugin name.
func (adapter *BufferedStorageSourceAdapter) Name() string {
	return "BufferedStorageSourceAdapter"
}

// Version returns the plugin version.
func (adapter *BufferedStorageSourceAdapter) Version() string {
	return "1.0.0"
}

// Type indicates this is a Source plugin.
func (adapter *BufferedStorageSourceAdapter) Type() pluginapi.PluginType {
	return pluginapi.SourcePlugin
}

// verifyPipeline checks if the pipeline is properly configured
func (adapter *BufferedStorageSourceAdapter) verifyPipeline() error {
	if len(adapter.processors) == 0 {
		return errors.New("no processors registered in pipeline")
	}

	log.Printf("Pipeline verification: Found %d processors", len(adapter.processors))
	for i, proc := range adapter.processors {
		log.Printf("Pipeline processor %d: %T", i, proc)
	}

	return nil
}

// Initialize reads the configuration map and sets up the adapter.
func (adapter *BufferedStorageSourceAdapter) Initialize(config map[string]interface{}) error {
	// Helper function to safely convert interface{} to int
	getIntValue := func(v interface{}) (int, bool) {
		switch i := v.(type) {
		case int:
			return i, true
		case float64:
			return int(i), true
		case int64:
			return int(i), true
		}
		return 0, false
	}

	// Get required config values.
	startLedgerRaw, ok := config["start_ledger"]
	if !ok {
		return errors.New("start_ledger must be specified")
	}
	startLedgerInt, ok := getIntValue(startLedgerRaw)
	if !ok {
		return errors.New("invalid start_ledger value")
	}
	startLedger := uint32(startLedgerInt)

	bucketName, ok := config["bucket_name"].(string)
	if !ok {
		return errors.New("bucket_name is missing")
	}

	network, ok := config["network"].(string)
	if !ok {
		return errors.New("network must be specified")
	}

	// Get other config values with defaults.
	bufferSizeInt, _ := getIntValue(config["buffer_size"])
	if bufferSizeInt == 0 {
		bufferSizeInt = 1024
	}
	numWorkersInt, _ := getIntValue(config["num_workers"])
	if numWorkersInt == 0 {
		numWorkersInt = 10
	}
	retryLimitInt, _ := getIntValue(config["retry_limit"])
	if retryLimitInt == 0 {
		retryLimitInt = 3
	}
	retryWaitInt, _ := getIntValue(config["retry_wait"])
	if retryWaitInt == 0 {
		retryWaitInt = 5
	}

	// End ledger is optional.
	endLedgerRaw, ok := config["end_ledger"]
	var endLedger uint32
	if ok {
		endLedgerInt, ok := getIntValue(endLedgerRaw)
		if !ok {
			return errors.New("invalid end_ledger value")
		}
		endLedger = uint32(endLedgerInt)
		if endLedger > 0 && endLedger < startLedger {
			return errors.New("end_ledger must be greater than start_ledger")
		}
	}

	// Optional: ledgers per file and files per partition.
	ledgersPerFileInt, _ := getIntValue(config["ledgers_per_file"])
	if ledgersPerFileInt == 0 {
		ledgersPerFileInt = 64
	}
	filesPerPartitionInt, _ := getIntValue(config["files_per_partition"])
	if filesPerPartitionInt == 0 {
		filesPerPartitionInt = 10
	}

	adapter.config = BufferedStorageConfig{
		BucketName:        bucketName,
		Network:           network,
		BufferSize:        uint32(bufferSizeInt),
		NumWorkers:        uint32(numWorkersInt),
		RetryLimit:        uint32(retryLimitInt),
		RetryWait:         uint32(retryWaitInt),
		StartLedger:       startLedger,
		EndLedger:         endLedger,
		LedgersPerFile:    uint32(ledgersPerFileInt),
		FilesPerPartition: uint32(filesPerPartitionInt),
	}

	log.Printf("BufferedStorageSourceAdapter initialized with start_ledger=%d, end_ledger=%d, bucket=%s, network=%s",
		startLedger, endLedger, bucketName, network)

	// Add pipeline verification after initialization
	if err := adapter.verifyPipeline(); err != nil {
		log.Printf("Warning: Pipeline verification failed: %v", err)
		// Optionally return the error if you want to fail initialization
		// return err
	}

	return nil
}

// Subscribe registers a processor to receive messages.
func (adapter *BufferedStorageSourceAdapter) Subscribe(proc pluginapi.Processor) {
	adapter.processors = append(adapter.processors, proc)
}

// Start begins the processing loop.
func (adapter *BufferedStorageSourceAdapter) Start(ctx context.Context) error {

	log.Printf("Starting BufferedStorageSourceAdapter with config: %+v", adapter.config)

	if err := adapter.verifyPipeline(); err != nil {
		return fmt.Errorf("pipeline verification failed: %w", err)
	}

	// Create schema configuration
	schema := datastore.DataStoreSchema{
		LedgersPerFile:    adapter.config.LedgersPerFile,
		FilesPerPartition: adapter.config.FilesPerPartition,
	}

	log.Printf("Created schema configuration: %+v", schema)

	// Create data store configuration
	dataStoreConfig := datastore.DataStoreConfig{
		Type:   "GCS",
		Schema: schema,
		Params: map[string]string{
			"destination_bucket_path": adapter.config.BucketName,
		},
	}

	log.Printf("Attempting to connect to GCS bucket: %s", adapter.config.BucketName)

	log.Printf("Starting BufferedStorageSourceAdapter from ledger %d", adapter.config.StartLedger)
	if adapter.config.EndLedger > 0 {
		log.Printf("Will process until ledger %d", adapter.config.EndLedger)
	} else {
		log.Printf("Will process indefinitely from start ledger")
	}

	// Add debug logging for configuration
	log.Printf("Using configuration: %+v", adapter.config)
	log.Printf("Number of registered processors: %d", len(adapter.processors))

	// Create buffered storage configuration.
	bufferedConfig := cdp.DefaultBufferedStorageBackendConfig(schema.LedgersPerFile)
	bufferedConfig.BufferSize = adapter.config.BufferSize
	bufferedConfig.NumWorkers = adapter.config.NumWorkers
	bufferedConfig.RetryLimit = adapter.config.RetryLimit
	bufferedConfig.RetryWait = time.Duration(adapter.config.RetryWait) * time.Second

	publisherConfig := cdp.PublisherConfig{
		DataStoreConfig:       dataStoreConfig,
		BufferedStorageConfig: bufferedConfig,
	}

	log.Printf("Created DataStore configuration: %+v", dataStoreConfig)
	log.Printf("Created buffered configuration: %+v", bufferedConfig)
	log.Printf("Created publisher configuration: %+v", publisherConfig)

	// Determine ledger range.
	var ledgerRange ledgerbackend.Range
	if adapter.config.EndLedger > 0 {
		ledgerRange = ledgerbackend.BoundedRange(
			adapter.config.StartLedger,
			adapter.config.EndLedger,
		)
	} else {
		ledgerRange = ledgerbackend.UnboundedRange(adapter.config.StartLedger)
	}

	log.Printf("BufferedStorageSourceAdapter: processing ledger range: %v", ledgerRange)

	processedLedgers := 0
	lastLogTime := time.Now()
	lastLedgerTime := time.Now()

	// Add a ticker for periodic status updates even if no ledgers are being processed
	statusTicker := time.NewTicker(10 * time.Second)
	defer statusTicker.Stop()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-statusTicker.C:
				log.Printf("Still alive - Processed %d ledgers so far", processedLedgers)
			}
		}
	}()

	err := cdp.ApplyLedgerMetadata(
		ledgerRange,
		publisherConfig,
		ctx,
		func(lcm xdr.LedgerCloseMeta) error {

			currentTime := time.Now()
			ledgerProcessingTime := currentTime.Sub(lastLedgerTime)
			lastLedgerTime = currentTime

			log.Printf("Processing ledger %d (time since last ledger: %v)", lcm.LedgerSequence(), ledgerProcessingTime)
			if err := adapter.processLedger(ctx, lcm); err != nil {
				log.Printf("Error processing ledger %d: %v", lcm.LedgerSequence(), err)
				return err
			}

			processedLedgers++
			if time.Since(lastLogTime) > 10*time.Second {
				rate := float64(processedLedgers) / time.Since(lastLogTime).Seconds()
				log.Printf("Processed %d ledgers (%.2f ledgers/sec)", processedLedgers, rate)
				lastLogTime = time.Now()
			}
			return nil
		},
	)

	if err != nil {
		log.Printf("BufferedStorageSourceAdapter encountered an error: %v", err)
		return err
	}

	duration := time.Since(lastLogTime)
	rate := float64(processedLedgers) / duration.Seconds()
	log.Printf("BufferedStorageSourceAdapter completed. Processed %d ledgers in %v (%.2f ledgers/sec)", processedLedgers, duration, rate)
	return nil
}

// processLedger processes each ledger by passing it to registered processors.
func (adapter *BufferedStorageSourceAdapter) processLedger(ctx context.Context, ledger xdr.LedgerCloseMeta) error {
	sequence := ledger.LedgerSequence()
	log.Printf("Starting to process ledger %d", sequence)

	// Check if we have any processors
	if len(adapter.processors) == 0 {
		log.Printf("Warning: No processors registered for ledger %d", sequence)
		return nil
	}

	// Create message once for all processors
	msg := pluginapi.Message{
		Payload:   ledger,
		Timestamp: time.Now(),
	}

	// Process through each processor in sequence
	for i, proc := range adapter.processors {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			procStart := time.Now()

			// Add processor-specific context
			processorCtx := context.WithValue(ctx, "processor_index", i)
			processorCtx = context.WithValue(processorCtx, "processor_type", fmt.Sprintf("%T", proc))

			if err := proc.Process(processorCtx, msg); err != nil {
				log.Printf("Error in processor %d (%T) for ledger %d: %v", i, proc, sequence, err)
				// You might want to implement retry logic here
				return errors.Wrapf(err, "processor %d (%T) failed", i, proc)
			}

			processingTime := time.Since(procStart)
			if processingTime > time.Second {
				log.Printf("Warning: Processor %d (%T) took %v to process ledger %d",
					i, proc, processingTime, sequence)
			} else {
				log.Printf("Processor %d (%T) successfully processed ledger %d in %v",
					i, proc, sequence, processingTime)
			}
		}
	}

	log.Printf("Successfully completed processing ledger %d through %d processors",
		sequence, len(adapter.processors))
	return nil
}

// Stop halts the adapter. For this example, it simply returns nil.
func (adapter *BufferedStorageSourceAdapter) Stop() error {
	// Implement any necessary cleanup here.
	log.Println("BufferedStorageSourceAdapter stopped")
	return nil
}

// Close is a convenience alias for Stop.
func (adapter *BufferedStorageSourceAdapter) Close() error {
	return adapter.Stop()
}

// Exported New function to allow dynamic loading.
func New() pluginapi.Plugin {
	// Return a new instance. Configuration will be provided via Initialize.
	return &BufferedStorageSourceAdapter{}
}
