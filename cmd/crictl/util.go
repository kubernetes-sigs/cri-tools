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

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/invopop/jsonschema"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/protoadapt"
	"google.golang.org/protobuf/runtime/protoiface"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/rest"
	internalapi "k8s.io/cri-api/pkg/apis"
	pb "k8s.io/cri-api/pkg/apis/runtime/v1"
	"sigs.k8s.io/yaml"
)

const (
	// truncatedImageIDLen is the truncated length of imageID.
	truncatedIDLen = 13

	outputTypeJSON       = "json"
	outputTypeYAML       = "yaml"
	outputTypeTable      = "table"
	outputTypeGoTemplate = "go-template"
)

var (
	// The global stopCh for monitoring Interrupt signal.
	// DO NOT use it directly. Use SetupInterruptSignalHandler() to get it.
	signalIntStopCh chan struct{}
	// only setup stopCh once.
	signalIntSetupOnce = &sync.Once{}
)

// SetupInterruptSignalHandler setup a global signal handler monitoring Interrupt signal. e.g: Ctrl+C.
// The returned read-only channel will be closed on receiving Interrupt signals.
// It will directly call os.Exit(1) on receiving Interrupt signal twice.
func SetupInterruptSignalHandler() <-chan struct{} {
	signalIntSetupOnce.Do(func() {
		signalIntStopCh = make(chan struct{})
		c := make(chan os.Signal, 2)
		signal.Notify(c, shutdownSignals...)

		go func() {
			<-c
			close(signalIntStopCh)
			<-c
			os.Exit(1) // Exit immediately on second signal
		}()
	})

	return signalIntStopCh
}

func InterruptableRPC[T any](
	ctx context.Context,
	rpcFunc func(context.Context) (T, error),
) (res T, err error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	resCh := make(chan T, 1)
	errCh := make(chan error, 1)

	go func() {
		res, err := rpcFunc(ctx)
		if err != nil {
			errCh <- err

			return
		}

		resCh <- res
	}()

	select {
	case <-SetupInterruptSignalHandler():
		cancel()

		return res, fmt.Errorf("interrupted: %w", ctx.Err())
	case err := <-errCh:
		return res, err
	case res := <-resCh:
		return res, nil
	}
}

type listOptions struct {
	// id of container or sandbox
	id string
	// podID of container
	podID string
	// Regular expression pattern to match pod or container
	nameRegexp string
	// Regular expression pattern to match the pod namespace
	podNamespaceRegexp string
	// state of the sandbox
	state string
	// show verbose info for the sandbox
	verbose bool
	// labels are selectors for the sandbox
	labels map[string]string
	// quiet is for listing just container/sandbox/image IDs
	quiet bool
	// output format
	output string
	// all containers
	all bool
	// latest container
	latest bool
	// last n containers
	last int
	// out with truncating the id
	noTrunc bool
	// image used by the container
	image string
	// resolve image path
	resolveImagePath bool
}

type execOptions struct {
	// id of container
	id string
	// timeout to stop command
	timeout int64
	// Whether to exec a command in a tty
	tty bool
	// Whether to stream stdin
	stdin bool
	// Command to exec
	cmd []string
	// transport to be used
	transport string
	// TLS configuration for streaming
	tlsConfig *rest.TLSClientConfig
}

type attachOptions struct {
	// id of container
	id string
	// Whether the stdin is TTY
	tty bool
	// Whether pass Stdin to container
	stdin bool
	// transport to be used
	transport string
	// TLS configuration for streaming
	tlsConfig *rest.TLSClientConfig
}

type portforwardOptions struct {
	// id of sandbox
	id string
	// ports to forward
	ports []string
	// transport to be used
	transport string
	// TLS configuration for streaming
	tlsConfig *rest.TLSClientConfig
}

func getSortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	return keys
}

func loadContainerConfig(path string) (*pb.ContainerConfig, error) {
	f, err := openFile(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var config pb.ContainerConfig
	if err := utilyaml.NewYAMLOrJSONDecoder(f, 4096).Decode(&config); err != nil {
		return nil, err
	}

	if config.GetMetadata() == nil {
		return nil, errors.New("metadata is not set")
	}

	if config.GetMetadata().GetName() == "" {
		return nil, fmt.Errorf("name is not in metadata %q", config.GetMetadata())
	}

	return &config, nil
}

func printJSONSchema(value any) error {
	schema := jsonschema.Reflect(value)

	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal JSON schema: %w", err)
	}

	fmt.Println(string(data))

	return nil
}

func loadPodSandboxConfig(path string) (*pb.PodSandboxConfig, error) {
	f, err := openFile(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var config pb.PodSandboxConfig
	if err := utilyaml.NewYAMLOrJSONDecoder(f, 4096).Decode(&config); err != nil {
		return nil, err
	}

	if config.GetMetadata() == nil {
		return nil, errors.New("metadata is not set")
	}

	if config.GetMetadata().GetUid() == "" {
		config.Metadata.Uid = uuid.New().String()
	}

	if config.GetMetadata().GetName() == "" || config.GetMetadata().GetNamespace() == "" {
		return nil, fmt.Errorf("name or namespace is not in metadata %q", config.GetMetadata())
	}

	if config.GetLinux() != nil && config.GetLinux().GetCgroupParent() == "" {
		logrus.Warn("cgroup_parent is not set. Use `runtime-config` to get the runtime cgroup driver")
	}

	return &config, nil
}

func openFile(path string) (*os.File, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("config at %s not found", path)
		}

		return nil, err
	}

	return f, nil
}

func protobufObjectToJSON(obj protoiface.MessageV1) (string, error) {
	msg := protoadapt.MessageV2Of(obj)

	marshaledJSON, err := protojson.MarshalOptions{EmitDefaultValues: true, Indent: "  "}.Marshal(msg)
	if err != nil {
		return "", err
	}

	return string(marshaledJSON), nil
}

func outputProtobufObjAsJSON(obj protoiface.MessageV1) error {
	marshaledJSON, err := protobufObjectToJSON(obj)
	if err != nil {
		return err
	}

	fmt.Println(marshaledJSON)

	return nil
}

func outputProtobufObjAsYAML(obj protoiface.MessageV1) error {
	marshaledJSON, err := protobufObjectToJSON(obj)
	if err != nil {
		return err
	}

	marshaledYAML, err := yaml.JSONToYAML([]byte(marshaledJSON))
	if err != nil {
		return err
	}

	fmt.Println(string(marshaledYAML))

	return nil
}

type statusData struct {
	json            string
	runtimeHandlers string
	features        string
	info            map[string]string
}

func outputStatusData(statuses []statusData, format, tmplStr string) (err error) {
	if len(statuses) == 0 {
		return nil
	}

	result := []map[string]any{}

	for _, status := range statuses {
		// Sort all keys
		keys := []string{}
		for k := range status.info {
			keys = append(keys, k)
		}

		sort.Strings(keys)

		infoMap := map[string]any{}

		if status.json != "" {
			var statusVal map[string]any

			err := json.Unmarshal([]byte(status.json), &statusVal)
			if err != nil {
				return fmt.Errorf("unmarshal status JSON: %w", err)
			}

			infoMap["status"] = statusVal
		}

		if status.runtimeHandlers != "" {
			var handlersVal []*any

			err := json.Unmarshal([]byte(status.runtimeHandlers), &handlersVal)
			if err != nil {
				return fmt.Errorf("unmarshal runtime handlers: %w", err)
			}

			if handlersVal != nil {
				infoMap["runtimeHandlers"] = handlersVal
			}
		}

		if status.features != "" {
			featuresVal := map[string]any{}

			err := json.Unmarshal([]byte(status.features), &featuresVal)
			if err != nil {
				return fmt.Errorf("unmarshal features JSON: %w", err)
			}

			if featuresVal != nil {
				infoMap["features"] = featuresVal
			}
		}

		for _, k := range keys {
			val := status.info[k]

			if strings.HasPrefix(val, "{") {
				// Assume a JSON object
				var genericVal map[string]any
				if err := json.Unmarshal([]byte(val), &genericVal); err != nil {
					return fmt.Errorf("unmarshal status info JSON: %w", err)
				}

				infoMap[k] = genericVal
			} else {
				// Assume a string and remove any double quotes
				infoMap[k] = strings.Trim(val, `"`)
			}
		}

		result = append(result, infoMap)
	}

	// Old behavior: single entries are not encapsulated within an array
	var jsonResult []byte
	if len(result) == 1 {
		jsonResult, err = json.Marshal(result[0])
	} else {
		jsonResult, err = json.Marshal(result)
	}

	if err != nil {
		return fmt.Errorf("marshal result: %w", err)
	}

	switch format {
	case outputTypeYAML:
		yamlInfo, err := yaml.JSONToYAML(jsonResult)
		if err != nil {
			return fmt.Errorf("JSON result to YAML: %w", err)
		}

		fmt.Println(string(yamlInfo))
	case outputTypeJSON:
		output := getJSONBuffer()
		defer putJSONBuffer(output)

		if err := json.Indent(output, jsonResult, "", "  "); err != nil {
			return fmt.Errorf("indent JSON result: %w", err)
		}

		fmt.Println(output.String())
	case outputTypeGoTemplate:
		output, err := tmplExecuteRawJSON(tmplStr, string(jsonResult))
		if err != nil {
			return fmt.Errorf("execute template: %w", err)
		}

		fmt.Println(output)
	default:
		return fmt.Errorf("unsupported format: %q", format)
	}

	return nil
}

func outputEvent(event protoiface.MessageV1, format, tmplStr string) error {
	switch format {
	case outputTypeYAML:
		err := outputProtobufObjAsYAML(event)
		if err != nil {
			return err
		}
	case outputTypeJSON:
		err := outputProtobufObjAsJSON(event)
		if err != nil {
			return err
		}
	case outputTypeGoTemplate:
		jsonEvent, err := protobufObjectToJSON(event)
		if err != nil {
			return err
		}

		output, err := tmplExecuteRawJSON(tmplStr, jsonEvent)
		if err != nil {
			return err
		}

		fmt.Println(output)
	default:
		fmt.Printf("Don't support %q format\n", format)
	}

	return nil
}

func parseLabelStringSlice(ss []string) (map[string]string, error) {
	labels := make(map[string]string)

	for _, s := range ss {
		pair := strings.Split(s, "=")
		if len(pair) != 2 {
			return nil, fmt.Errorf("incorrectly specified label: %v", s)
		}

		labels[pair[0]] = pair[1]
	}

	return labels, nil
}

// marshalMapInOrder marshals a map into JSON in the order of the original
// data structure.
func marshalMapInOrder(m map[string]any, t any) (string, error) {
	var sb strings.Builder
	sb.WriteString("{")

	v := reflect.ValueOf(t)
	numFields := v.Type().NumField()
	fieldCount := 0

	for i := range numFields {
		field := jsonFieldFromTag(v.Type().Field(i).Tag)
		if field == "" || field == "-" {
			continue
		}

		value, err := json.Marshal(m[field])
		if err != nil {
			return "", err
		}

		if fieldCount > 0 {
			sb.WriteString(",")
		}

		fmt.Fprintf(&sb, "%q:%s", field, value)

		fieldCount++
	}

	sb.WriteString("}")

	buf := getJSONBuffer()
	defer putJSONBuffer(buf)

	if err := json.Indent(buf, []byte(sb.String()), "", "  "); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// jsonFieldFromTag gets json field name from field tag.
func jsonFieldFromTag(tag reflect.StructTag) string {
	field := strings.Split(tag.Get(outputTypeJSON), ",")[0]

	for f := range strings.SplitSeq(tag.Get("protobuf"), ",") {
		if !strings.HasPrefix(f, "json=") {
			continue
		}

		field = strings.TrimPrefix(f, "json=")
	}

	return field
}

func getTruncatedID(id, prefix string) string {
	id = strings.TrimPrefix(id, prefix)
	if len(id) > truncatedIDLen {
		id = id[:truncatedIDLen]
	}

	return id
}

func matchesRegex(pattern, target string) bool {
	if pattern == "" {
		return true
	}

	matched, err := regexp.MatchString(pattern, target)
	if err != nil {
		// Assume it's not a match if an error occurs.
		return false
	}

	return matched
}

func matchesImage(ctx context.Context, imageClient internalapi.ImageManagerService, image, containerImage string) (bool, error) {
	if image == "" || imageClient == nil {
		return true, nil
	}

	r1, err := ImageStatus(ctx, imageClient, image, false)
	if err != nil {
		return false, err
	}

	r2, err := ImageStatus(ctx, imageClient, containerImage, false)
	if err != nil {
		return false, err
	}

	if r1.GetImage() == nil || r2.GetImage() == nil {
		// Always return not match if the image doesn't exist.
		return false, nil
	}

	return r1.GetImage().GetId() == r2.GetImage().GetId(), nil
}

func getRepoImage(ctx context.Context, imageClient internalapi.ImageManagerService, image string) (string, error) {
	r, err := ImageStatus(ctx, imageClient, image, false)
	if err != nil {
		return "", err
	}

	if len(r.GetImage().GetRepoTags()) > 0 {
		return r.GetImage().GetRepoTags()[0], nil
	}

	return image, nil
}

func handleDisplay(
	ctx context.Context,
	client internalapi.RuntimeService,
	watch bool,
	displayFunc func(context.Context, internalapi.RuntimeService) error,
) error {
	if !watch {
		return displayFunc(ctx, client)
	}

	displayErrCh := make(chan error, 1)

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	watchCtx, cancelFn := context.WithCancel(ctx)
	defer cancelFn()

	// Put the displayPodMetrics in another goroutine, because it might be
	// time consuming with lots of pods and we want to cancel it
	// ASAP when user hit CtrlC
	go func() {
		for range ticker.C {
			if err := displayFunc(watchCtx, client); err != nil {
				displayErrCh <- err

				break
			}
		}
	}()

	// listen for CtrlC or error
	select {
	case <-SetupInterruptSignalHandler():
		cancelFn()

		return nil
	case err := <-displayErrCh:
		return err
	}
}
