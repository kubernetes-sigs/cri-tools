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

package validate

import (
	"context"
	"errors"
	"fmt"
	"net"
	"slices"
	"sync"
	"time"

	nri "github.com/containerd/nri/pkg/api"
	"github.com/containerd/nri/pkg/stub"

	"sigs.k8s.io/cri-tools/pkg/framework"
)

// NRIEventType represents the type of NRI lifecycle event.
type NRIEventType string

const (
	EventRunPodSandbox    NRIEventType = "RunPodSandbox"
	EventStopPodSandbox   NRIEventType = "StopPodSandbox"
	EventRemovePodSandbox NRIEventType = "RemovePodSandbox"
	EventCreateContainer  NRIEventType = "CreateContainer"
	EventStartContainer   NRIEventType = "StartContainer"
	EventStopContainer    NRIEventType = "StopContainer"
	EventRemoveContainer  NRIEventType = "RemoveContainer"
)

// NRIEvent records an NRI lifecycle event with metadata.
type NRIEvent struct {
	Type          NRIEventType
	PodSandboxID  string
	PodName       string
	PodNamespace  string
	PodUID        string
	ContainerID   string
	ContainerName string
	Timestamp     time.Time
}

// NRITestPlugin implements the NRI plugin interfaces for testing.
// It records all events and supports optional hook callbacks for blocking/error injection.
type NRITestPlugin struct {
	mu     sync.Mutex
	events []NRIEvent

	// ready is closed when Synchronize is called (after register/configure handshake).
	ready     chan struct{}
	readyOnce sync.Once

	// syncPods/syncContainers capture the pod and container IDs passed to the
	// most recent Synchronize call, so tests can assert that a late-joining
	// plugin is reconciled with the runtime's existing state.
	syncPods       []string
	syncContainers []string

	// Hook callbacks - if set, called during the respective hook.
	// Return an error to simulate plugin failure.
	//
	// OnSynchronize fires during the Synchronize handshake (before the ready
	// channel is closed), so it must be installed before the stub connects
	// (see the configure callback on StartNRITestStub). Block inside it to hold
	// the Synchronize call open while the test mutates runtime state.
	OnSynchronize      func(ctx context.Context, pods []*nri.PodSandbox, containers []*nri.Container) error
	OnRunPodSandbox    func(ctx context.Context, pod *nri.PodSandbox) error
	OnStopPodSandbox   func(ctx context.Context, pod *nri.PodSandbox) error
	OnRemovePodSandbox func(ctx context.Context, pod *nri.PodSandbox) error
	OnCreateContainer  func(ctx context.Context, pod *nri.PodSandbox, container *nri.Container) error
	OnStartContainer   func(ctx context.Context, pod *nri.PodSandbox, container *nri.Container) error
	OnStopContainer    func(ctx context.Context, pod *nri.PodSandbox, container *nri.Container) error
	OnRemoveContainer  func(ctx context.Context, pod *nri.PodSandbox, container *nri.Container) error
}

// Synchronize implements stub.SynchronizeInterface.
// It captures the existing pods/containers the runtime reconciles the plugin
// with, then signals readiness (registration/configuration complete) via the
// ready channel.
func (p *NRITestPlugin) Synchronize(ctx context.Context, pods []*nri.PodSandbox, containers []*nri.Container) ([]*nri.ContainerUpdate, error) {
	p.mu.Lock()

	p.syncPods = make([]string, 0, len(pods))
	for _, pod := range pods {
		p.syncPods = append(p.syncPods, pod.GetId())
	}

	p.syncContainers = make([]string, 0, len(containers))
	for _, container := range containers {
		p.syncContainers = append(p.syncContainers, container.GetId())
	}

	p.mu.Unlock()

	// Invoke the optional hook while still inside the Synchronize call. A test
	// may block here to keep the late-joining plugin in its synchronization
	// window (the runtime has not yet observed the plugin as ready) while it
	// creates additional containers, exercising the race where a container
	// created during Synchronize must not be lost.
	if p.OnSynchronize != nil {
		if err := p.OnSynchronize(ctx, pods, containers); err != nil {
			return nil, err
		}
	}

	p.readyOnce.Do(func() { close(p.ready) })

	return nil, nil
}

// SyncedPods returns the pod sandbox IDs passed to the most recent Synchronize call.
func (p *NRITestPlugin) SyncedPods() []string {
	p.mu.Lock()
	defer p.mu.Unlock()

	result := make([]string, len(p.syncPods))
	copy(result, p.syncPods)

	return result
}

// SyncedContainers returns the container IDs passed to the most recent Synchronize call.
func (p *NRITestPlugin) SyncedContainers() []string {
	p.mu.Lock()
	defer p.mu.Unlock()

	result := make([]string, len(p.syncContainers))
	copy(result, p.syncContainers)

	return result
}

// RunPodSandbox implements stub.RunPodInterface.
func (p *NRITestPlugin) RunPodSandbox(ctx context.Context, pod *nri.PodSandbox) error {
	p.recordPodEvent(EventRunPodSandbox, pod)

	if p.OnRunPodSandbox != nil {
		return p.OnRunPodSandbox(ctx, pod)
	}

	return nil
}

// StopPodSandbox implements stub.StopPodInterface.
func (p *NRITestPlugin) StopPodSandbox(ctx context.Context, pod *nri.PodSandbox) error {
	p.recordPodEvent(EventStopPodSandbox, pod)

	if p.OnStopPodSandbox != nil {
		return p.OnStopPodSandbox(ctx, pod)
	}

	return nil
}

// RemovePodSandbox implements stub.RemovePodInterface.
func (p *NRITestPlugin) RemovePodSandbox(ctx context.Context, pod *nri.PodSandbox) error {
	p.recordPodEvent(EventRemovePodSandbox, pod)

	if p.OnRemovePodSandbox != nil {
		return p.OnRemovePodSandbox(ctx, pod)
	}

	return nil
}

// CreateContainer implements stub.CreateContainerInterface.
func (p *NRITestPlugin) CreateContainer(ctx context.Context, pod *nri.PodSandbox, container *nri.Container) (*nri.ContainerAdjustment, []*nri.ContainerUpdate, error) {
	p.recordContainerEvent(EventCreateContainer, pod, container)

	if p.OnCreateContainer != nil {
		if err := p.OnCreateContainer(ctx, pod, container); err != nil {
			return nil, nil, err
		}
	}

	return nil, nil, nil
}

// StartContainer implements stub.StartContainerInterface.
func (p *NRITestPlugin) StartContainer(ctx context.Context, pod *nri.PodSandbox, container *nri.Container) error {
	p.recordContainerEvent(EventStartContainer, pod, container)

	if p.OnStartContainer != nil {
		return p.OnStartContainer(ctx, pod, container)
	}

	return nil
}

// StopContainer implements stub.StopContainerInterface.
func (p *NRITestPlugin) StopContainer(ctx context.Context, pod *nri.PodSandbox, container *nri.Container) ([]*nri.ContainerUpdate, error) {
	p.recordContainerEvent(EventStopContainer, pod, container)

	if p.OnStopContainer != nil {
		if err := p.OnStopContainer(ctx, pod, container); err != nil {
			return nil, err
		}
	}

	return nil, nil
}

// RemoveContainer implements stub.RemoveContainerInterface.
func (p *NRITestPlugin) RemoveContainer(ctx context.Context, pod *nri.PodSandbox, container *nri.Container) error {
	p.recordContainerEvent(EventRemoveContainer, pod, container)

	if p.OnRemoveContainer != nil {
		return p.OnRemoveContainer(ctx, pod, container)
	}

	return nil
}

// Events returns a copy of all recorded events.
func (p *NRITestPlugin) Events() []NRIEvent {
	p.mu.Lock()
	defer p.mu.Unlock()

	result := make([]NRIEvent, len(p.events))
	copy(result, p.events)

	return result
}

// Reset clears all recorded events.
func (p *NRITestPlugin) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.events = nil
	p.syncPods = nil
	p.syncContainers = nil
}

// LastRunPodSandboxID returns the pod sandbox ID from the most recent RunPodSandbox event,
// or empty string if none recorded. This is useful for cleanup in AfterEach when the test
// may have failed before capturing the pod ID from the CRI call.
func (p *NRITestPlugin) LastRunPodSandboxID() string {
	p.mu.Lock()
	defer p.mu.Unlock()

	for i := range slices.Backward(p.events) {
		if p.events[i].Type == EventRunPodSandbox {
			return p.events[i].PodSandboxID
		}
	}

	return ""
}

// WaitForEventCount waits until at least count events are recorded, or times out.
func (p *NRITestPlugin) WaitForEventCount(count int, timeout time.Duration) ([]NRIEvent, error) {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		p.mu.Lock()

		if len(p.events) >= count {
			result := make([]NRIEvent, len(p.events))
			copy(result, p.events)
			p.mu.Unlock()

			return result, nil
		}

		p.mu.Unlock()
		time.Sleep(50 * time.Millisecond)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	return nil, fmt.Errorf("timed out waiting for %d NRI events after %v (got %d)", count, timeout, len(p.events))
}

// FilterEventsByPodID returns events matching a specific pod sandbox ID.
func FilterEventsByPodID(events []NRIEvent, podID string) []NRIEvent {
	var filtered []NRIEvent

	for i := range events {
		if events[i].PodSandboxID == podID {
			filtered = append(filtered, events[i])
		}
	}

	return filtered
}

func (p *NRITestPlugin) recordPodEvent(eventType NRIEventType, pod *nri.PodSandbox) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.events = append(p.events, NRIEvent{
		Type:         eventType,
		PodSandboxID: pod.GetId(),
		PodName:      pod.GetName(),
		PodNamespace: pod.GetNamespace(),
		PodUID:       pod.GetUid(),
		Timestamp:    time.Now(),
	})
}

func (p *NRITestPlugin) recordContainerEvent(eventType NRIEventType, pod *nri.PodSandbox, container *nri.Container) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.events = append(p.events, NRIEvent{
		Type:          eventType,
		PodSandboxID:  pod.GetId(),
		PodName:       pod.GetName(),
		PodNamespace:  pod.GetNamespace(),
		PodUID:        pod.GetUid(),
		ContainerID:   container.GetId(),
		ContainerName: container.GetName(),
		Timestamp:     time.Now(),
	})
}

// NRITestStub wraps an NRI stub instance for test lifecycle management.
type NRITestStub struct {
	Plugin *NRITestPlugin
	Stub   stub.Stub
	cancel context.CancelFunc
	done   chan struct{}
}

// StartNRITestStub creates and starts an NRI test stub connected to the runtime.
//
// Optional configure callbacks run against the plugin before it connects, so
// tests can install hooks that fire during the registration/Synchronize
// handshake (e.g. OnSynchronize), which cannot be set after this call returns
// because the handshake has already completed by then.
func StartNRITestStub(pluginName, pluginIdx string, configure ...func(*NRITestPlugin)) (*NRITestStub, error) {
	socketPath := framework.TestContext.NRISocketPath
	if socketPath == "" {
		return nil, errors.New("NRI socket path not configured")
	}

	plugin := &NRITestPlugin{
		ready: make(chan struct{}),
	}

	for _, c := range configure {
		c(plugin)
	}
	// Use a custom dialer to capture the underlying network connection.
	// If Start() gets stuck (e.g., waiting for Configure that never arrives),
	// we can force-close the connection to free network resources.
	var (
		conn   net.Conn
		connMu sync.Mutex
	)

	dialer := func(path string) (net.Conn, error) {
		c, err := (&net.Dialer{}).DialContext(context.Background(), "unix", path)
		if err != nil {
			return nil, err
		}

		connMu.Lock()
		conn = c
		connMu.Unlock()

		return c, nil
	}

	s, err := stub.New(plugin,
		stub.WithPluginName(pluginName),
		stub.WithPluginIdx(pluginIdx),
		stub.WithSocketPath(socketPath),
		stub.WithDialer(dialer),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create NRI stub: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	errCh := make(chan error, 1)

	go func() {
		defer close(done)

		errCh <- s.Run(ctx)
	}()

	// Wait for the stub to complete registration and configuration with the runtime.
	// plugin.ready is closed when Synchronize is called (after register/configure handshake).
	select {
	case <-done:
		cancel()

		return nil, fmt.Errorf("NRI stub exited early: %w", <-errCh)
	case <-plugin.ready:
		// Registration and configuration complete
	case <-time.After(10 * time.Second):
		cancel()
		// Don't block indefinitely on <-done: stub.Start() may be stuck on an
		// internal channel read (cfgErrC) that doesn't observe context cancellation.
		select {
		case <-done:
			// Goroutine finished; safe to read the error.
			return nil, fmt.Errorf("NRI stub did not become ready within 10s: %w", <-errCh)
		case <-time.After(5 * time.Second):
			// Goroutine still running — likely stuck in Start()'s <-cfgErrC read,
			// which does not observe context cancellation. Force-close the
			// underlying connection to break the ttrpc multiplex and unblock
			// the server goroutine. This prevents a leaked connection from
			// remaining open and potentially tainting later serial NRI specs.
			connMu.Lock()
			if conn != nil {
				conn.Close()
			}
			connMu.Unlock()
			// Use a bounded wait for the goroutine to exit. If Start() is truly
			// stuck on <-cfgErrC (which conn.Close() may not unblock), we accept
			// the goroutine leak rather than blocking the test indefinitely.
			go func() {
				select {
				case <-done:
					s.Stop()
				case <-time.After(30 * time.Second):
					// Permanently stuck — attempt Stop() anyway to release resources,
					// then accept the goroutine leak.
					framework.Logf("NRI stub goroutine still running after 30s; calling Stop() and accepting leak")
					s.Stop()
				}
			}()

			return nil, errors.New("NRI stub did not become ready within 10s and failed to stop within 5s")
		}
	}

	return &NRITestStub{
		Plugin: plugin,
		Stub:   s,
		cancel: cancel,
		done:   done,
	}, nil
}

// Stop disconnects the NRI stub from the runtime.
func (ts *NRITestStub) Stop() {
	// Cancel context first to unblock Run()'s select on ctx.Done().
	// Run() will call stub.Stop() internally, which closes connections and
	// waits for in-flight handlers. If a handler is blocked on a test channel,
	// closing connections should unblock it via gRPC cancellation.
	ts.cancel()

	select {
	case <-ts.done:
	case <-time.After(5 * time.Second):
		// Timeout: Run() is stuck (e.g., handler blocked on test channel).
		// Nothing more we can do - proceed with cleanup.
	}
}

// Cleanup stops the stub and resets events.
func (ts *NRITestStub) Cleanup() {
	ts.Stop()
	ts.Plugin.Reset()
}
