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
	"bytes"
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

	"github.com/golang/protobuf/jsonpb" //nolint:staticcheck
	"github.com/golang/protobuf/proto"  //nolint:staticcheck
	"github.com/invopop/jsonschema"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
	internalapi "k8s.io/cri-api/pkg/apis"
	pb "k8s.io/cri-api/pkg/apis/runtime/v1"
	"sigs.k8s.io/yaml"
)

const (
	// truncatedImageIDLen is the truncated length of imageID
	truncatedIDLen = 13
)

var (
	// The global stopCh for monitoring Interrupt signal.
	// DO NOT use it directly. Use SetupInterruptSignalHandler() to get it.
	signalIntStopCh chan struct{}
	// only setup stopCh once
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
	if ctx == nil {
		ctx = context.Background()
	}
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
}

type portforwardOptions struct {
	// id of sandbox
	id string
	// ports to forward
	ports []string
	// transport to be used
	transport string
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

	if config.Metadata == nil {
		return nil, errors.New("metadata is not set")
	}

	if config.Metadata.Name == "" {
		return nil, fmt.Errorf("name is not in metadata %q", config.Metadata)
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

	if config.Metadata == nil {
		return nil, errors.New("metadata is not set")
	}

	if config.Metadata.Name == "" || config.Metadata.Namespace == "" || config.Metadata.Uid == "" {
		return nil, fmt.Errorf("name, namespace or uid is not in metadata %q", config.Metadata)
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

func protobufObjectToJSON(obj proto.Message) (string, error) {
	jsonpbMarshaler := jsonpb.Marshaler{EmitDefaults: true, Indent: "  "}
	marshaledJSON, err := jsonpbMarshaler.MarshalToString(obj)
	if err != nil {
		return "", err
	}
	return marshaledJSON, nil
}

func outputProtobufObjAsJSON(obj proto.Message) error {
	marshaledJSON, err := protobufObjectToJSON(obj)
	if err != nil {
		return err
	}

	fmt.Println(marshaledJSON)
	return nil
}

func outputProtobufObjAsYAML(obj proto.Message) error {
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

func outputStatusInfo(status, handlers string, info map[string]string, format string, tmplStr string) error {
	// Sort all keys
	keys := []string{}
	for k := range info {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	infoMap := map[string]any{}

	if status != "" {
		var statusVal map[string]any
		err := json.Unmarshal([]byte(status), &statusVal)
		if err != nil {
			return err
		}
		infoMap["status"] = statusVal
	}

	if handlers != "" {
		var handlersVal []*any
		err := json.Unmarshal([]byte(handlers), &handlersVal)
		if err != nil {
			return err
		}
		if handlersVal != nil {
			infoMap["runtimeHandlers"] = handlersVal
		}
	}

	for _, k := range keys {
		var genericVal map[string]any
		json.Unmarshal([]byte(info[k]), &genericVal)
		infoMap[k] = genericVal
	}

	jsonInfo, err := json.Marshal(infoMap)
	if err != nil {
		return err
	}

	switch format {
	case "yaml":
		yamlInfo, err := yaml.JSONToYAML(jsonInfo)
		if err != nil {
			return err
		}
		fmt.Println(string(yamlInfo))
	case "json":
		var output bytes.Buffer
		if err := json.Indent(&output, jsonInfo, "", "  "); err != nil {
			return err
		}
		fmt.Println(output.String())
	case "go-template":
		output, err := tmplExecuteRawJSON(tmplStr, string(jsonInfo))
		if err != nil {
			return err
		}
		fmt.Println(output)
	default:
		fmt.Printf("Don't support %q format\n", format)
	}
	return nil
}

func outputEvent(event proto.Message, format string, tmplStr string) error {
	switch format {
	case "yaml":
		err := outputProtobufObjAsYAML(event)
		if err != nil {
			return err
		}
	case "json":
		err := outputProtobufObjAsJSON(event)
		if err != nil {
			return err
		}
	case "go-template":
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

// marshalMapInOrder marshalls a map into json in the order of the original
// data structure.
func marshalMapInOrder(m map[string]interface{}, t interface{}) (string, error) {
	s := "{"
	v := reflect.ValueOf(t)
	for i := 0; i < v.Type().NumField(); i++ {
		field := jsonFieldFromTag(v.Type().Field(i).Tag)
		if field == "" || field == "-" {
			continue
		}
		value, err := json.Marshal(m[field])
		if err != nil {
			return "", err
		}
		s += fmt.Sprintf("%q:%s,", field, value)
	}
	s = s[:len(s)-1]
	s += "}"
	var buf bytes.Buffer
	if err := json.Indent(&buf, []byte(s), "", "  "); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// jsonFieldFromTag gets json field name from field tag.
func jsonFieldFromTag(tag reflect.StructTag) string {
	field := strings.Split(tag.Get("json"), ",")[0]
	for _, f := range strings.Split(tag.Get("protobuf"), ",") {
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

func matchesImage(imageClient internalapi.ImageManagerService, image string, containerImage string) (bool, error) {
	if image == "" {
		return true, nil
	}
	r1, err := ImageStatus(imageClient, image, false)
	if err != nil {
		return false, err
	}
	r2, err := ImageStatus(imageClient, containerImage, false)
	if err != nil {
		return false, err
	}
	if r1.Image == nil || r2.Image == nil {
		// Always return not match if the image doesn't exist.
		return false, nil
	}
	return r1.Image.Id == r2.Image.Id, nil
}

func getRepoImage(imageClient internalapi.ImageManagerService, image string) (string, error) {
	r, err := ImageStatus(imageClient, image, false)
	if err != nil {
		return "", err
	}
	if len(r.Image.RepoTags) > 0 {
		return r.Image.RepoTags[0], nil
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
