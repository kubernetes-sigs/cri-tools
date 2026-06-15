# NRI Spec Discrepancies

This document tracks cases where observed runtime behavior differs from the
NRI Pod Sandbox Lifecycle specification (containerd/nri#286).

## StopPodSandbox idempotency — sandbox reuse after Stop

**Spec requirement** (from containerd/nri#286): After StopPodSandbox, the
sandbox is never reused — CreateContainer on a stopped sandbox MUST fail.

**Observed behavior**: containerd allows CreateContainer to succeed on a
stopped sandbox, returning a valid container ID instead of an error.

**Contract points in this scenario:**

- [x] StopPodSandbox is idempotent (multiple calls succeed without error)
- [x] StopPodSandbox NRI hook fires at least once
- [ ] CreateContainer on a stopped sandbox fails ← discrepancy

**Impact**: A plugin that assumes stopped sandboxes are immutable may see
unexpected container lifecycle events after StopPodSandbox.

**Status**: Open

## Multi-plugin teardown hook delivery after plugin failure

**Spec requirement** (from containerd/nri#286): One plugin's failure on
StopPodSandbox/RemovePodSandbox MUST NOT prevent delivery of those hooks to
subsequent plugins.

**Observed behavior**: NRI aborts hook delivery to remaining plugins when one
plugin returns an error. Plugin 1 (higher index) receives zero
StopPodSandbox/RemovePodSandbox hooks when plugin 0 (lower index) returns an
error.

**Contract points in this scenario:**

- [x] Plugin 0 receives StopPodSandbox hook
- [x] Plugin 0 receives RemovePodSandbox hook
- [ ] Plugin 1 receives StopPodSandbox hook despite plugin 0's error ← discrepancy
- [ ] Plugin 1 receives RemovePodSandbox hook despite plugin 0's error ← discrepancy
- [x] StopPodSandbox CRI call succeeds despite plugin error
- [x] RemovePodSandbox CRI call succeeds despite plugin error
- [x] Sandbox is fully removed

**Impact**: A lower-index plugin returning an error on teardown silently
prevents all higher-index plugins from receiving teardown notifications,
potentially causing resource leaks in those plugins.

**Status**: Open

## containerd: NRI plugin errors swallowed on StopPodSandbox CRI call

**Spec requirement**: StopPodSandbox and RemovePodSandbox CRI calls MUST
propagate NRI plugin errors to the CRI caller so the caller is aware of
the failure.

**Observed behavior** (containerd): When an NRI plugin returns an error on
StopPodSandbox or RemovePodSandbox, containerd swallows the error and
returns success to the CRI caller. CRI-O correctly propagates the error.

**Affected runtimes**: containerd (CRI-O handles this correctly)

**Impact**: The CRI caller is unaware that an NRI plugin failed during
teardown, potentially masking resource leaks or plugin state corruption.

**Status**: Open

## CRI-O: empty container name in NRI CreateContainer event metadata

**Spec requirement**: NRI CreateContainer events should include the container
name in the event metadata so plugins can identify containers.

**Observed behavior** (CRI-O): The `ContainerName` field in NRI
CreateContainer events is an empty string.

**Affected runtimes**: CRI-O (containerd populates this correctly)

**Impact**: Plugins that rely on container name for policy decisions cannot
identify containers during CreateContainer hooks on CRI-O.

**Status**: Open
