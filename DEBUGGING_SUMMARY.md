# Debugging Summary for balraj-cos-pvc-2

## What We Found

### ✅ PVC Configuration is CORRECT
```bash
$ kubectl get pvc balraj-cos-pvc-2 -o yaml | grep accessModes -A 1
spec:
  accessModes:
  - ReadOnlyMany  # ✅ Correctly set to ReadOnlyMany
```

### ✅ PV Shows ReadOnlyMany (ROX)
```bash
$ kubectl get pv | grep balraj-cos-pvc-2
pvc-25ebb694-d91d-4621-8142-f2623a50e8e5   256Mi      ROX  # ✅ ROX = ReadOnlyMany
```

### ❌ Problem: Driver Doesn't Declare MULTI_NODE_READER_ONLY Support

**Current Code** in `pkg/driver/s3-driver.go:32-34`:
```go
volumeCapabilities = []csi.VolumeCapability_AccessMode_Mode{
    csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,  // Only this!
}
```

## Root Cause Analysis

1. **PVC requests ReadOnlyMany** ✅
2. **Kubernetes translates to MULTI_NODE_READER_ONLY** ✅
3. **Driver doesn't declare support for MULTI_NODE_READER_ONLY** ❌
4. **Kubernetes falls back to SINGLE_NODE_WRITER mode** ❌
5. **Sets readonly=false in CSI request** ❌
6. **Volume mounted as read-write instead of read-only** ❌

## Why readonly=false in Logs

When you see in logs:
```
readonly: false
```

It's because:
- Kubernetes checked if driver supports `MULTI_NODE_READER_ONLY`
- Driver only declares `SINGLE_NODE_WRITER`
- Kubernetes uses `SINGLE_NODE_WRITER` mode instead
- For `SINGLE_NODE_WRITER`, readonly is always `false`

## The Fix

We need to:

1. **Add MULTI_NODE_READER_ONLY capability** to driver
2. **Check the access mode** in NodePublishVolume
3. **Pass readonly flag** to mounter via secretMap
4. **Add `-o ro`** to s3fs mount command

## Implementation Status

- [x] Diagnosed the issue
- [x] Confirmed PVC has ReadOnlyMany
- [x] Confirmed PV shows ROX
- [x] Identified missing capability in driver
- [ ] Implement the fix (3 files to modify)
- [ ] Test with the existing PVC

## Next Steps

Implement the fix in 3 files:
1. `pkg/driver/s3-driver.go` - Add MULTI_NODE_READER_ONLY capability
2. `pkg/driver/nodeserver.go` - Check access mode and set ro flag
3. `pkg/mounter/mounter-s3fs.go` - Handle ro flag in mount options

After implementation, the existing PVC `balraj-cos-pvc-2` will automatically use readonly mode on next mount.