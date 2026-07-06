# Debugging: Why is readonly=false for balraj-cos-pvc-2?

## Step 1: Check the PVC Configuration

Run this command to see the actual PVC configuration:

```bash
kubectl get pvc balraj-cos-pvc-2 -o yaml
```

**Look for these fields**:
```yaml
spec:
  accessModes:
    - ReadWriteOnce    # or ReadWriteMany or ReadOnlyMany?
  storageClassName: <storage-class-name>
  resources:
    requests:
      storage: <size>
```

## Step 2: Check the Pod Using This PVC

Find which pod is using this PVC:

```bash
kubectl get pods -o json | jq -r '.items[] | select(.spec.volumes[]?.persistentVolumeClaim.claimName=="balraj-cos-pvc-2") | .metadata.name'
```

Then check the pod's volume mount configuration:

```bash
kubectl get pod <pod-name> -o yaml
```

**Look for**:
```yaml
spec:
  containers:
  - name: <container-name>
    volumeMounts:
    - name: <volume-name>
      mountPath: /path
      readOnly: true   # <-- Is this set?
  volumes:
  - name: <volume-name>
    persistentVolumeClaim:
      claimName: balraj-cos-pvc-2
      readOnly: true   # <-- Or is this set?
```

## Step 3: Check Driver's Supported Capabilities

Check what capabilities your driver currently declares:

```bash
# Get the CSI driver pod
kubectl get pods -n kube-system | grep ibm-object-csi

# Check the driver logs for capability registration
kubectl logs -n kube-system <csi-driver-pod> | grep -i "volume.*capability\|access.*mode"
```

**Expected output should include**:
```
Enabling volume access mode: SINGLE_NODE_WRITER
Enabling volume access mode: MULTI_NODE_READER_ONLY  # <-- This should be present
```

## Step 4: Check CSI Driver Info

```bash
kubectl get csidriver
kubectl get csidriver <driver-name> -o yaml
```

Look for `volumeLifecycleModes` and supported features.

## Step 5: Check the Actual CSI Request

Enable debug logging and check what Kubernetes is sending:

```bash
# Get CSI node pod logs with debug level
kubectl logs -n kube-system <csi-node-pod> --tail=100 | grep -A 20 "NodePublishVolume.*balraj-cos-pvc-2"
```

**Look for these fields in the logs**:
```
readonly: false  # <-- Current value
accessMode: <mode>  # <-- What mode is being used?
VolumeCapability: <details>
```

## Common Scenarios and Diagnosis

### Scenario A: PVC has ReadWriteOnce or ReadWriteMany

**PVC Config**:
```yaml
spec:
  accessModes:
    - ReadWriteOnce  # or ReadWriteMany
```

**Result**: `readonly = false` ✅ **This is CORRECT behavior**

**Why**: These access modes are for read-write access, so readonly should be false.

**Solution**: If you want readonly, change PVC to:
```yaml
spec:
  accessModes:
    - ReadOnlyMany
```

### Scenario B: PVC has ReadOnlyMany but Driver Doesn't Support It

**PVC Config**:
```yaml
spec:
  accessModes:
    - ReadOnlyMany
```

**Driver Capabilities** (in s3-driver.go):
```go
volumeCapabilities = []csi.VolumeCapability_AccessMode_Mode{
    csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,  // Only this!
    // Missing: MULTI_NODE_READER_ONLY
}
```

**Result**: `readonly = false` ❌ **This is the BUG**

**Why**: 
1. Kubernetes sees PVC wants ReadOnlyMany
2. Checks if driver supports MULTI_NODE_READER_ONLY
3. Driver doesn't declare this capability
4. Kubernetes falls back to SINGLE_NODE_WRITER
5. Sets readonly = false

**Solution**: Add MULTI_NODE_READER_ONLY to driver capabilities (see implementation plan).

### Scenario C: Pod Mount Override

**PVC Config**:
```yaml
spec:
  accessModes:
    - ReadWriteMany
```

**Pod Config**:
```yaml
spec:
  containers:
  - volumeMounts:
    - name: my-vol
      mountPath: /data
      readOnly: true  # <-- This should set readonly=true
```

**Result**: Should be `readonly = true`, but might be `false` if Kubernetes version is old.

## Quick Diagnostic Commands

Run these commands and share the output:

```bash
# 1. Get PVC details
echo "=== PVC Configuration ==="
kubectl get pvc balraj-cos-pvc-2 -o yaml | grep -A 5 "accessModes:\|spec:"

# 2. Get Pod using this PVC
echo "=== Pod Using PVC ==="
POD_NAME=$(kubectl get pods -o json | jq -r '.items[] | select(.spec.volumes[]?.persistentVolumeClaim.claimName=="balraj-cos-pvc-2") | .metadata.name' | head -1)
echo "Pod: $POD_NAME"
kubectl get pod $POD_NAME -o yaml | grep -A 10 "volumeMounts:\|volumes:"

# 3. Get CSI Driver Logs
echo "=== CSI Driver Logs for this PVC ==="
kubectl logs -n kube-system $(kubectl get pods -n kube-system -l app=ibm-object-csi-node -o name | head -1) --tail=200 | grep -A 15 "balraj-cos-pvc-2"

# 4. Check Driver Capabilities
echo "=== Driver Capabilities ==="
kubectl logs -n kube-system $(kubectl get pods -n kube-system -l app=ibm-object-csi-node -o name | head -1) | grep -i "Enabling volume access mode"
```

## Expected vs Actual Analysis

| Check | Expected | Actual | Issue? |
|-------|----------|--------|--------|
| PVC accessModes | ReadOnlyMany | ? | Run: `kubectl get pvc balraj-cos-pvc-2 -o yaml \| grep accessModes -A 1` |
| Pod readOnly mount | true (optional) | ? | Run: `kubectl get pod <pod> -o yaml \| grep readOnly` |
| Driver supports MULTI_NODE_READER_ONLY | Yes | No (currently) | **This is the root cause** |
| CSI Request readonly field | true | false | **This is the symptom** |

## Root Cause Determination

Based on the CSI spec and your current code:

**Most Likely Cause**: Your driver doesn't declare `MULTI_NODE_READER_ONLY` capability in [`pkg/driver/s3-driver.go:32-34`](pkg/driver/s3-driver.go:32-34).

**Current Code**:
```go
volumeCapabilities = []csi.VolumeCapability_AccessMode_Mode{
    csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
}
```

**What Happens**:
1. You create PVC with `accessModes: [ReadOnlyMany]`
2. Kubernetes checks driver capabilities
3. Driver only supports `SINGLE_NODE_WRITER`
4. Kubernetes either:
   - Binds PVC using `SINGLE_NODE_WRITER` mode (readonly=false)
   - OR rejects the PVC entirely

**Fix**: Add the missing capability:
```go
volumeCapabilities = []csi.VolumeCapability_AccessMode_Mode{
    csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
    csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY,  // Add this!
}
```

## Next Steps

1. **Run the diagnostic commands above** and share the output
2. **Check the PVC's accessModes** - is it actually set to ReadOnlyMany?
3. **Verify driver capabilities** - does it log MULTI_NODE_READER_ONLY support?
4. **If driver doesn't support it** - implement the fix in the implementation plan

## Quick Test

To quickly test if this is the issue, try creating a NEW PVC with explicit ReadOnlyMany:

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: test-readonly-pvc
spec:
  accessModes:
    - ReadOnlyMany  # Explicit readonly
  storageClassName: <your-storage-class>
  resources:
    requests:
      storage: 1Gi
```

Then check if it binds successfully and what the logs show.