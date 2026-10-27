/*
Copyright The Kubernetes Authors.

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
	"errors"
	"flag"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/urfave/cli/v2"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	internalapi "k8s.io/cri-api/pkg/apis"
	pb "k8s.io/cri-api/pkg/apis/runtime/v1"
	"sigs.k8s.io/yaml"
)

type podCheckpointRuntime struct {
	internalapi.RuntimeService

	containers        []*pb.Container
	filter            *pb.ContainerFilter
	checkpointRequest *pb.CheckpointPodRequest
	restoreRequest    *pb.RestorePodRequest
	restoreResponse   *pb.RestorePodResponse
	listErr           error
	checkpointErr     error
	restoreErr        error
}

func (r *podCheckpointRuntime) ListContainers(
	_ context.Context,
	filter *pb.ContainerFilter,
) ([]*pb.Container, error) {
	r.filter = filter

	return r.containers, r.listErr
}

func (r *podCheckpointRuntime) CheckpointPod(
	_ context.Context,
	request *pb.CheckpointPodRequest,
) error {
	r.checkpointRequest = request

	return r.checkpointErr
}

func (r *podCheckpointRuntime) RestorePod(
	_ context.Context,
	request *pb.RestorePodRequest,
) (*pb.RestorePodResponse, error) {
	r.restoreRequest = request

	return r.restoreResponse, r.restoreErr
}

func podCheckpointCLIContext(
	t *testing.T,
	config *CrictlConfig,
	args ...string,
) (*cli.Context, *bytes.Buffer) {
	t.Helper()

	flags := flag.NewFlagSet("pod-checkpoint-test", flag.ContinueOnError)
	flags.String("path", "", "")
	flags.String("pod-config", "", "")
	flags.Var(cli.NewStringSlice(), "container-config", "")
	flags.String("runtime", "", "")
	flags.String("output", "", "")

	if err := flags.Parse(args); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer

	app := &cli.App{Metadata: map[string]any{configKey: config}, Writer: &output}
	ctx := cli.NewContext(app, flags, nil)
	ctx.Context = t.Context()

	return ctx, &output
}

func podRestoreTestArgs(t *testing.T) []string {
	t.Helper()

	dir := t.TempDir()

	configs := map[string]string{
		"pod.yaml":     "metadata: {name: pod, namespace: default, uid: pod-uid}\n",
		"app.yaml":     "metadata: {name: app}\nimage: {image: example/app:1}\n",
		"sidecar.yaml": "metadata: {name: sidecar}\nimage: {image: example/sidecar:1}\n",
	}
	for name, data := range configs {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	return []string{
		"--pod-config", filepath.Join(dir, "pod.yaml"),
		"--container-config", filepath.Join(dir, "app.yaml"),
		"--container-config", filepath.Join(dir, "sidecar.yaml"),
	}
}

func TestPodCheckpointCommandArguments(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name    string
		command *cli.Command
		args    []string
		wantErr string
	}{
		{name: "checkpoint missing ID", command: checkpointPodCommand, wantErr: "POD-ID cannot be empty"},
		{
			name: "checkpoint multiple IDs", command: checkpointPodCommand,
			args: []string{"--path", "/checkpoint", "one", "two"}, wantErr: "only one pod sandbox",
		},
		{
			name: "checkpoint missing path", command: checkpointPodCommand,
			args: []string{"pod"}, wantErr: "without specifying the checkpoint path",
		},
		{name: "restore missing path", command: restorePodCommand, wantErr: "CHECKPOINT-PATH cannot be empty"},
		{
			name: "restore multiple paths", command: restorePodCommand,
			args: []string{"/one", "/two"}, wantErr: "only one checkpoint path",
		},
		{
			name: "restore missing pod config", command: restorePodCommand,
			args: []string{"/checkpoint"}, wantErr: "--pod-config is required",
		},
		{
			name: "restore missing container configs", command: restorePodCommand,
			args: []string{"--pod-config", "unused", "/checkpoint"}, wantErr: "--container-config is required",
		},
		{
			name: "restore invalid output", command: restorePodCommand,
			args:    []string{"--pod-config", "unused", "--container-config", "unused", "--output", "xml", "/checkpoint"},
			wantErr: "unsupported output format",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctx, output := podCheckpointCLIContext(
				t,
				&CrictlConfig{RuntimeEndpointIsSet: true},
				test.args...)
			g := NewWithT(t)
			g.Expect(test.command.Action(ctx)).To(MatchError(ContainSubstring(test.wantErr)))
			g.Expect(output.Len()).To(BeZero())
		})
	}
}

func TestCheckpointPodCommand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		containers    []*pb.Container
		listErr       error
		checkpointErr error
		wantErr       string
	}{
		{
			name:       "running containers",
			containers: []*pb.Container{{Id: "app-id"}, {Id: "sidecar-id"}},
		},
		{name: "no running containers", wantErr: "has no running containers"},
		{
			name:    "list failure",
			listErr: errors.New("list failed"),
			wantErr: "listing running containers",
		},
		{
			name:          "checkpoint failure",
			containers:    []*pb.Container{{Id: "app-id"}, {Id: "sidecar-id"}},
			checkpointErr: errors.New("checkpoint failed"),
			wantErr:       "checkpointing pod",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			g := NewWithT(t)
			runtime := &podCheckpointRuntime{
				containers:    test.containers,
				listErr:       test.listErr,
				checkpointErr: test.checkpointErr,
			}
			ctx, output := podCheckpointCLIContext(
				t,
				&CrictlConfig{runtimeServiceOverride: runtime},
				"--path",
				"/checkpoint",
				"sandbox-id",
			)

			err := checkpointPodCommand.Action(ctx)
			if test.wantErr != "" {
				g.Expect(err).To(MatchError(ContainSubstring(test.wantErr)))
				g.Expect(output.Len()).To(BeZero())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output.String()).To(Equal("Pod sandbox-id checkpointed to /checkpoint\n"))
			}

			g.Expect(runtime.filter.GetPodSandboxId()).To(Equal("sandbox-id"))
			g.Expect(runtime.filter.GetState().GetState()).
				To(Equal(pb.ContainerState_CONTAINER_RUNNING))

			if len(test.containers) == 0 {
				g.Expect(runtime.checkpointRequest).To(BeNil())

				return
			}

			g.Expect(runtime.checkpointRequest.GetPodSandboxId()).To(Equal("sandbox-id"))
			g.Expect(runtime.checkpointRequest.GetOutputPath()).To(Equal("/checkpoint"))
			g.Expect(runtime.checkpointRequest.GetContainerIds()).
				To(Equal([]string{"app-id", "sidecar-id"}))
		})
	}
}

func TestRestorePodCommandOutputsContainerIDs(t *testing.T) {
	t.Parallel()

	for _, format := range []string{"", outputTypeJSON, outputTypeYAML} {
		t.Run("format="+format, func(t *testing.T) {
			t.Parallel()

			g := NewWithT(t)
			runtime := &podCheckpointRuntime{restoreResponse: &pb.RestorePodResponse{
				PodSandboxId: "restored-pod",
				RestoredContainers: []*pb.RestoredContainer{
					{Name: "sidecar", ContainerId: "restored-sidecar"},
					{Name: "app", ContainerId: "restored-app"},
				},
			}}

			args := append(
				podRestoreTestArgs(t),
				"--runtime",
				"custom",
				"--output",
				format,
				"/checkpoint",
			)
			ctx, output := podCheckpointCLIContext(
				t,
				&CrictlConfig{runtimeServiceOverride: runtime},
				args...)
			g.Expect(restorePodCommand.Action(ctx)).To(Succeed())
			g.Expect(runtime.restoreRequest.GetCheckpointPath()).To(Equal("/checkpoint"))
			g.Expect(runtime.restoreRequest.GetRuntimeHandler()).To(Equal("custom"))
			g.Expect(runtime.restoreRequest.GetConfig().GetMetadata().GetUid()).To(Equal("pod-uid"))
			g.Expect(runtime.restoreRequest.GetContainerConfigs()).To(HaveLen(2))
			g.Expect(runtime.restoreRequest.GetContainerConfigs()[0].GetMetadata().GetName()).
				To(Equal("app"))
			g.Expect(runtime.restoreRequest.GetContainerConfigs()[1].GetMetadata().GetName()).
				To(Equal("sidecar"))

			if format == "" {
				g.Expect(output.String()).To(Equal("Pod restored: restored-pod\n"))

				return
			}

			data := output.Bytes()

			if format == outputTypeYAML {
				var err error

				data, err = yaml.YAMLToJSON(data)
				g.Expect(err).NotTo(HaveOccurred())
			}

			var response pb.RestorePodResponse
			g.Expect(protojson.Unmarshal(data, &response)).To(Succeed())
			g.Expect(proto.Equal(&response, runtime.restoreResponse)).To(BeTrue())
		})
	}
}

func TestRestorePodCommandValidatesConfigsBeforeConnection(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name    string
		change  func([]string) []string
		wantErr string
	}{
		{
			name: "missing pod config", change: func(args []string) []string {
				args[1] += ".missing"

				return args
			}, wantErr: "failed to load pod sandbox config",
		},
		{
			name: "missing container config", change: func(args []string) []string {
				args[3] += ".missing"

				return args
			}, wantErr: "failed to load container config",
		},
		{
			name: "duplicate container name", change: func(args []string) []string {
				args[5] = args[3]

				return args
			}, wantErr: "duplicated in restore configs",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			args := append(test.change(podRestoreTestArgs(t)), "/checkpoint")
			ctx, output := podCheckpointCLIContext(
				t,
				&CrictlConfig{RuntimeEndpointIsSet: true},
				args...)
			g := NewWithT(t)
			g.Expect(restorePodCommand.Action(ctx)).To(MatchError(ContainSubstring(test.wantErr)))
			g.Expect(output.Len()).To(BeZero())
		})
	}
}

func TestRestorePodCommandRequiresExplicitUID(t *testing.T) {
	t.Parallel()

	for name, metadata := range map[string]string{
		"missing UID": "metadata: {name: pod, namespace: default}\n",
		"empty UID":   "metadata: {name: pod, namespace: default, uid: ''}\n",
		"null UID":    "metadata: {name: pod, namespace: default, uid: null}\n",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			args := podRestoreTestArgs(t)
			g := NewWithT(t)
			g.Expect(os.WriteFile(args[1], []byte(metadata), 0o600)).To(Succeed())
			args = append(args, "/checkpoint")
			ctx, output := podCheckpointCLIContext(
				t,
				&CrictlConfig{RuntimeEndpointIsSet: true},
				args...)
			g.Expect(restorePodCommand.Action(ctx)).
				To(MatchError(ContainSubstring("metadata.uid is required for restore")))
			g.Expect(output.Len()).To(BeZero())
		})
	}
}

func TestRestorePodCommandRejectsInvalidResponses(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name       string
		response   *pb.RestorePodResponse
		restoreErr error
		wantErr    string
	}{
		{name: "RPC failure", restoreErr: errors.New("restore failed"), wantErr: "restoring pod"},
		{name: "nil response", wantErr: "pod sandbox ID is empty"},
		{name: "empty sandbox ID", response: &pb.RestorePodResponse{}, wantErr: "pod sandbox ID is empty"},
		{name: "missing containers", response: &pb.RestorePodResponse{PodSandboxId: "pod"}, wantErr: "returned 0 containers"},
		{
			name: "unexpected name", response: &pb.RestorePodResponse{
				PodSandboxId: "pod", RestoredContainers: []*pb.RestoredContainer{
					{Name: "other", ContainerId: "one"}, {Name: "sidecar", ContainerId: "two"},
				},
			}, wantErr: "unexpected or duplicate container name",
		},
		{
			name: "duplicate name", response: &pb.RestorePodResponse{
				PodSandboxId: "pod", RestoredContainers: []*pb.RestoredContainer{
					{Name: "app", ContainerId: "one"}, {Name: "app", ContainerId: "two"},
				},
			}, wantErr: "unexpected or duplicate container name",
		},
		{
			name: "empty container ID", response: &pb.RestorePodResponse{
				PodSandboxId: "pod", RestoredContainers: []*pb.RestoredContainer{
					{Name: "app"}, {Name: "sidecar", ContainerId: "two"},
				},
			}, wantErr: "empty ID for container",
		},
		{
			name: "duplicate container ID", response: &pb.RestorePodResponse{
				PodSandboxId: "pod", RestoredContainers: []*pb.RestoredContainer{
					{Name: "app", ContainerId: "one"}, {Name: "sidecar", ContainerId: "one"},
				},
			}, wantErr: "duplicate container ID",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			runtime := &podCheckpointRuntime{
				restoreResponse: test.response,
				restoreErr:      test.restoreErr,
			}

			args := append(podRestoreTestArgs(t), "--output", "json", "/checkpoint")
			ctx, output := podCheckpointCLIContext(
				t,
				&CrictlConfig{runtimeServiceOverride: runtime},
				args...)
			g := NewWithT(t)
			g.Expect(restorePodCommand.Action(ctx)).To(MatchError(ContainSubstring(test.wantErr)))
			g.Expect(output.Len()).To(BeZero())
		})
	}
}
