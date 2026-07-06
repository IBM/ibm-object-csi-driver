# When is NodePublishVolumeRequest.Readonly True or False?

## CSI Spec Definition

From the CSI spec (`csi.pb.go` line 4110-4112):

```go
// Indicates SP MUST publish the volume in readonly mode.
// This field is REQUIRED.
Readonly bool `protobuf:"varint,6,opt,name=readonly,proto3" json:"readonly,omitempty"`
```

**Key Point**: The comment says "Indicates SP MUST publish the volume in readonly mode" - this is set by the **Container Orchestrator (CO)**, which is Kubernetes in our case.

## How Kubernetes Sets the Readonly Field

According to the CSI specification, Kubernetes (the CO) sets the `readonly` field based on **TWO factors**:

### Factor 1: VolumeCapability AccessMode

The `VolumeCapability.AccessMode` from the PVC's `accessModes` field:

| Kubernetes PVC accessMode | CSI VolumeCapability_AccessMode | Readonly Field |
|---------------------------|----------------------------------|----------------|
| `ReadWriteOnce` | `SINGLE_NODE_WRITER` | **false** |
| `ReadWriteMany` | `MULTI_NODE_MULTI_WRITER` | **false** |
| `ReadOnlyMany` | `MULTI_NODE_READER_ONLY` | **true** |
| `ReadWriteOncePod` | `SINGLE_NODE_SINGLE_WRITER` | **false** |

### Factor 2: Pod's volumeMount.readOnly

In the Pod spec, you can also set `readOnly: true` on individual volume mounts:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: test-pod
spec:
  containers:
  - name: test-container
    volumeMounts:
    - name: my-volume
      mountPath: /data
      readOnly: true  # <-- This also sets readonly=true
  volumes:
  - name: my-volume
    persistentVolumeClaim:
      claimName: my-pvc
```

## When Readonly is TRUE

The `readonly` field in `NodePublishVolumeRequest` is set to **TRUE** when:

1. **PVC has `accessModes: [ReadOnlyMany]`**
   - Kubernetes translates this to `VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY`
   - Sets `readonly = true` in the CSI request

2. **Pod's volumeMount has `readOnly: true`**
   - Even if PVC is `ReadWriteOnce` or `ReadWriteMany`
   - Kubernetes sets `readonly = true` in the CSI request
   - This is a **per-mount** override

3. **Both conditions above**
   - If PVC is ReadOnlyMany AND Pod mount is readOnly: true
   - Result: `readonly = true`

## When Readonly is FALSE

The `readonly` field is **FALSE** when:

1. **PVC has `accessModes: [ReadWriteOnce]` or `[ReadWriteMany]`**
   - AND Pod's volumeMount does NOT have `readOnly: true`
   - Result: `readonly = false`

## Example Scenarios

### Scenario 1: ReadOnlyMany PVC
```yaml
# PVC
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: my-pvc
spec:
  accessModes:
    - ReadOnlyMany  # <-- This sets readonly=true
  resources:
    requests:
      storage: 10Gi
```

**Result**: `NodePublishVolumeRequest.Readonly = true`

### Scenario 2: ReadWriteMany PVC with readOnly mount
```yaml
# PVC
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: my-pvc
spec:
  accessModes:
    - ReadWriteMany  # <-- This would normally be readonly=false
  resources:
    requests:
      storage: 10Gi

---
# Pod
apiVersion: v1
kind: Pod
metadata:
  name: my-pod
spec:
  containers:
  - name: my-container
    volumeMounts:
    - name: my-volume
      mountPath: /data
      readOnly: true  # <-- But this overrides it to readonly=true
  volumes:
  - name: my-volume
    persistentVolumeClaim:
      claimName: my-pvc
```

**Result**: `NodePublishVolumeRequest.Readonly = true`

### Scenario 3: ReadWriteOnce PVC (normal case)
```yaml
# PVC
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: my-pvc
spec:
  accessModes:
    - ReadWriteOnce  # <-- This sets readonly=false
  resources:
    requests:
      storage: 10Gi
```

**Result**: `NodePublishVolumeRequest.Readonly = false`

## Why Check Both AccessMode AND Readonly?

In our implementation, we check BOTH:

```go
volumeCapability := req.GetVolumeCapability()
accessMode := volumeCapability.GetAccessMode().GetMode()
readOnly := req.GetReadonly()

// Check both conditions
if accessMode == csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY || readOnly {
    secretMap["ro"] = "true"
}
```

**Reasons**:

1. **AccessMode check**: Catches PVCs with `ReadOnlyMany`
2. **Readonly flag check**: Catches Pod-level `readOnly: true` overrides
3. **Defense in depth**: Ensures we handle all readonly scenarios

## Current Problem in Your Driver

Looking at your logs showing `readonly: false` when PVC has `ReadOnlyMany`:

**Possible Causes**:

1. **Driver doesn't declare MULTI_NODE_READER_ONLY capability**
   - If driver doesn't support this mode, Kubernetes might not set it correctly
   - Solution: Add `MULTI_NODE_READER_ONLY` to supported capabilities

2. **Kubernetes version issue**
   - Older Kubernetes versions might not properly translate ReadOnlyMany
   - Check Kubernetes version compatibility

3. **CSI driver version mismatch**
   - Ensure CSI spec version matches Kubernetes expectations

## Verification Steps

To verify what Kubernetes is sending:

```bash
# Enable debug logging in your CSI driver
kubectl logs <csi-node-pod> | grep -A 10 "NodePublishVolume"

# Look for these fields:
# - readonly: true/false
# - accessMode: MULTI_NODE_READER_ONLY
# - VolumeCapability details
```

## Summary

| Condition | Readonly Field Value |
|-----------|---------------------|
| PVC: ReadOnlyMany | **TRUE** |
| PVC: ReadWriteOnce/Many + Pod mount readOnly: true | **TRUE** |
| PVC: ReadWriteOnce/Many (normal) | **FALSE** |
| Driver doesn't support MULTI_NODE_READER_ONLY | **FALSE** (Kubernetes rejects or falls back) |

**Bottom Line**: The `readonly` field should be `true` when PVC has `ReadOnlyMany`, but your driver must first declare support for `MULTI_NODE_READER_ONLY` capability for Kubernetes to use it properly.