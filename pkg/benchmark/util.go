/*
Copyright 2021 The Kubernetes Authors.

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

package benchmark

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/sirupsen/logrus"
)

// LifecycleBenchmarkDatapoint encodes a single benchmark for a lifecycle operation.
// Contains a slice of int64s which represent the duration in nanoseconds of the
// operations which comprise a lifecycle being benchmarked.
// (e.g. the individual CRUD operations which are cycled through during the benchmark).
type LifecycleBenchmarkDatapoint struct {
	// int64 index of the sample.
	SampleIndex int `json:"sampleIndex"`

	// int64 nanosecond timestamp of the start of the result.
	StartTime int64 `json:"startTime"`

	// int64 nanosecond timestamp of the start of the result.
	EndTime int64 `json:"endTime"`

	// Slice of int64s representing the durations of individual operations.
	// The operation durations should be in the order they were executed in.
	// Note that the sum of OperationsDurationsNs need not be exactly equal to the duration
	// determined by subtracting StartTime from EndTime, as there may be additional steps
	// (e.g. timer setup) performed between the individual operations.
	OperationsDurationsNs []int64 `json:"operationsDurationsNs"`

	// String mapping for adding arbitrary meta-info for the lifecycle result:
	MetaInfo map[string]string `json:"metaInfo"`
}

// LifecycleBenchmarksResultsSet houses results for benchmarks involving resource lifecycles
// which include multiple benchmarked iterations of the cycle.
type LifecycleBenchmarksResultsSet struct {
	// Slice of string operation names which represent one cycle.
	OperationsNames []string `json:"operationsNames"`

	// The maximum number of lifecycles which were benchmarked in parallel.
	// Anything <= 1 should be considered sequential.
	NumParallel int `json:"numParallel"`

	// List of datapoints for each lifecycle benchmark.
	Datapoints []LifecycleBenchmarkDatapoint `json:"datapoints"`
}

// LifecycleBenchmarksResultsManager tracks lifecycle benchmark results through channels.
type LifecycleBenchmarksResultsManager struct {
	// The LifecycleBenchmarksResultsSet where results are added.
	resultsSet LifecycleBenchmarksResultsSet

	// Channel for sending results to the manager.
	resultsChannel chan *LifecycleBenchmarkDatapoint

	// Channel to indicate when the results consumer goroutine has ended.
	resultsOverChannel chan bool

	// Flag to indicate whether the results consumer goroutine is running.
	resultsConsumerRunning bool

	// The maximum timeout in seconds to wait between individual results being received.
	resultsChannelTimeoutSeconds int
}

// NewLifecycleBenchmarksResultsManager instantiates a new LifecycleBenchmarksResultsManager and its internal channels/structures.
func NewLifecycleBenchmarksResultsManager(initialResultsSet LifecycleBenchmarksResultsSet, resultsChannelTimeoutSeconds int) *LifecycleBenchmarksResultsManager {
	lbrm := LifecycleBenchmarksResultsManager{
		resultsSet:                   initialResultsSet,
		resultsChannelTimeoutSeconds: resultsChannelTimeoutSeconds,
		resultsChannel:               make(chan *LifecycleBenchmarkDatapoint),
		resultsOverChannel:           make(chan bool),
	}
	if lbrm.resultsSet.Datapoints == nil {
		lbrm.resultsSet.Datapoints = make([]LifecycleBenchmarkDatapoint, 0)
	}

	return &lbrm
}

// StartResultsConsumer starts the results consumer goroutine and returns the channel to write results to.
// A nil value must be sent after all other results were sent to indicate the end of the result
// stream.
func (lbrm *LifecycleBenchmarksResultsManager) StartResultsConsumer() chan *LifecycleBenchmarkDatapoint {
	if !lbrm.resultsConsumerRunning {
		lbrm.resultsConsumerRunning = true
		go lbrm.awaitResult()
	}

	return lbrm.resultsChannel
}

// AwaitAllResults waits for the result consumer goroutine and returns all the results registered insofar.
func (lbrm *LifecycleBenchmarksResultsManager) AwaitAllResults(timeoutSeconds int) error {
	if !lbrm.resultsConsumerRunning {
		return nil
	}

	timeout := time.After(time.Duration(timeoutSeconds) * time.Second)
	select {
	case <-lbrm.resultsOverChannel:
		lbrm.resultsConsumerRunning = false

		return nil
	case <-timeout:
		logrus.Warnf("Failed to await all results. Results registered so far were: %+v", lbrm.resultsSet)

		return fmt.Errorf("benchmark results waiting timed out after %d seconds", timeoutSeconds)
	}
}

// WriteResultsFile saves the results gathered so far as JSON under the given filepath.
func (lbrm *LifecycleBenchmarksResultsManager) WriteResultsFile(filepath string) error {
	if lbrm.resultsConsumerRunning {
		return errors.New("results consumer is still running and expecting results")
	}

	data, err := json.MarshalIndent(lbrm.resultsSet, "", " ")
	if err == nil {
		err = os.WriteFile(filepath, data, 0o644)
		if err != nil {
			return fmt.Errorf("failed to write benchmarks results to file: %v", filepath)
		}
	} else {
		return fmt.Errorf("failed to serialize benchmark data: %w", err)
	}

	return nil
}

// Function which continuously consumes results from the resultsChannel until receiving a nil.
func (lbrm *LifecycleBenchmarksResultsManager) awaitResult() {
	numOperations := len(lbrm.resultsSet.OperationsNames)

	for {
		var res *LifecycleBenchmarkDatapoint

		timeout := time.After(time.Duration(lbrm.resultsChannelTimeoutSeconds) * time.Second)

		select {
		case res = <-lbrm.resultsChannel:
			// Receiving nil indicates results are over:
			if res == nil {
				logrus.Info("Results ended")

				lbrm.resultsConsumerRunning = false
				lbrm.resultsOverChannel <- true

				return
			}

			// Warn if an improper number of results was received:
			if len(res.OperationsDurationsNs) != numOperations {
				logrus.Warnf("Received improper number of datapoints for operations %+v: %+v", lbrm.resultsSet.OperationsNames, res.OperationsDurationsNs)
			}

			// Register the result:
			lbrm.resultsSet.Datapoints = append(lbrm.resultsSet.Datapoints, *res)

		case <-timeout:
			err := fmt.Errorf("timed out after waiting %d seconds for new results", lbrm.resultsChannelTimeoutSeconds)
			logrus.Error(err.Error())
			panic(err)
		}
	}
}
