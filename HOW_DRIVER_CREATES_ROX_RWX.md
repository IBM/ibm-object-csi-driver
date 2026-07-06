# How Driver Creates ROX and RWX Volumes

## Question
How does the driver create PVs with ReadOnlyMany (ROX) and ReadWriteMany (RWX) access modes when it only declares `SINGLE_NODE_WRITER` capability?

---

## Answer: Kubernetes Creates the PV, Not the Driver

### Key Understanding

**The CSI driver does NOT create the PersistentVolume (PV) object in Kubernetes.**

Instead:
1. **Driver creates the storage backend** (S3 bucket)
2. **Kubernetes creates the PV object** with access modes from the PVC
3. **Driver must support the access mode** when mounting

---

## The Complete Flow

### 1. User Creates PVC

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: balraj-cos-pvc-2
spec:
  accessModes:
    - ReadOnlyMany  # User specifies ROX
  storageClassName: ibm-object-storage-standard-s3fs
  resources:
    requests:
      storage: 10Gi
```

### 2. Kubernetes CSI External Provisioner

The **external-provisioner** sidecar (not the driver itself) does the following:

```
1. Reads PVC with accessModes: [ReadOnlyMany]
2. Calls driver's CreateVolume() with VolumeCapabilities
3. Driver creates S3 bucket (storage backend)
4. Driver returns CreateVolumeResponse with VolumeContext
5. External-provisioner creates PV object with:
   - accessModes: [ReadOnlyMany] (from PVC)
   - volumeHandle: (from driver response)
   - volumeAttributes: (from driver response)
```

### 3. Driver's CreateVolume Response

**File:** `pkg/driver/controllerserver.go` (Lines 330-336)

```go
return &csi.CreateVolumeResponse{
    Volume: &csi.Volume{
        VolumeId:      volumeID,
        CapacityBytes: req.GetCapacityRange().GetRequiredBytes(),
        VolumeContext: params,  // Contains bucket info, endpoint, etc.
    },
}, nil
```

**Notice:** The driver does NOT set `AccessModes` in the response. The external-provisioner sets it based on the PVC.

---

## Why ROX/RWX PVs Exist Despite Driver Only Declaring RWO

### Current Situation

**Driver Capabilities** (`pkg/driver/s3-driver.go:30-34`):
```go
volumeCapabilities = []csi.VolumeCapability_AccessMode_Mode{
    csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,  // Only RWO
}
```

**But PVs exist with:**
- `ReadOnlyMany` (ROX) - Line 3172 in log
- `ReadWriteMany` (RWX) - Lines 53, 1042, 2971 in log

### How This Happens

1. **Kubernetes doesn't strictly enforce capability matching during PV creation**
   - The external-provisioner creates the PV with whatever access mode the PVC requests
   - The driver's declared capabilities are more of a "hint" than a hard constraint

2. **The real enforcement happens during mounting**
   - When kubelet tries to mount the volume, it calls `NodePublishVolume`
   - The driver receives the `VolumeCapability` with the access mode
   - If the driver doesn't support it, it should return an error

3. **Current driver doesn't validate access mode**
   - The driver accepts any access mode in `NodePublishVolume`
   - It doesn't check if the access mode is in its declared capabilities
   - This is why ROX/RWX mounts "work" (but incorrectly as RW)

---

## The Problem

### What Should Happen

```
PVC: ReadOnlyMany
     ↓
External-Provisioner creates PV with ROX
     ↓
kubelet calls NodePublishVolume with MULTI_NODE_READER_ONLY
     ↓
Driver checks: Is MULTI_NODE_READER_ONLY in my capabilities?
     ↓
YES → Mount with -o ro flag
NO  → Return error: "Unsupported access mode"
```

### What Actually Happens

```
PVC: ReadOnlyMany
     ↓
External-Provisioner creates PV with ROX
     ↓
kubelet calls NodePublishVolume with MULTI_NODE_READER_ONLY
     ↓
Driver: Ignores access mode, doesn't check capabilities ❌
     ↓
Mounts as read-write (WRONG!) ❌
```

---

## Evidence from Logs

### Log Line 6: Driver Startup
```
VolumeCapabilityAccessModes":[1]
```
- `1` = `SINGLE_NODE_WRITER` (RWO)
- Driver only declares RWO support

### Log Line 3172: NodePublishVolume Request
```
access_mode:{mode:MULTI_NODE_READER_ONLY}
```
- Kubernetes sends ROX access mode
- Driver receives it but doesn't handle it

### Result
- PV shows `ROX` in Kubernetes
- Driver mounts as read-write
- **Mismatch between PV access mode and actual mount behavior**

---

## How Other CSI Drivers Handle This

### Example: AWS EFS CSI Driver

**Declared Capabilities:**
```go
volumeCapabilities = []csi.VolumeCapability_AccessMode_Mode{
    csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
    csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER,
    csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY,
}
```

**In NodePublishVolume:**
```go
accessMode := req.GetVolumeCapability().GetAccessMode().GetMode()
switch accessMode {
case csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY:
    mountOptions = append(mountOptions, "ro")
case csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER:
    // Read-write mount
case csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER:
    // Read-write mount
default:
    return nil, status.Error(codes.InvalidArgument, "Unsupported access mode")
}
```

---

## The Solution for IBM Object CSI Driver

### Step 1: Declare Support for ROX and RWX

**File:** `pkg/driver/s3-driver.go`

```go
volumeCapabilities = []csi.VolumeCapability_AccessMode_Mode{
    csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
    csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER,  // Add RWX
    csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY,   // Add ROX
}
```

### Step 2: Handle Access Modes in NodePublishVolume

**File:** `pkg/driver/nodeserver.go` (After line 116)

```go
// Check access mode and set readonly flag if needed
accessMode := req.GetVolumeCapability().GetAccessMode().GetMode()
if accessMode == csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY {
    secretMap["ro"] = "true"
    klog.V(2).Infof("-NodePublishVolume-: Setting readonly mount for access mode: MULTI_NODE_READER_ONLY")
} else if readOnly {
    // Also respect the readonly boolean flag
    secretMap["ro"] = "true"
    klog.V(2).Infof("-NodePublishVolume-: Setting readonly mount from readonly flag")
}
```

### Step 3: Handle ro Flag in Mounter

**File:** `pkg/mounter/mounter-s3fs.go` (After line 247)

```go
// Add readonly flag if specified
if val, ok := secretMap["ro"]; ok && val == "true" {
    mountOptsMap["ro"] = ""  // ro flag has no value
    klog.V(2).Infof("Adding readonly flag to mount options")
}
```

---

## Why S3 Supports Both ROX and RWX

### S3 Characteristics

1. **Object Storage**: S3 is object storage, not block storage
2. **Concurrent Access**: Multiple clients can read/write simultaneously
3. **No File Locking**: S3 doesn't have traditional file locking
4. **Eventually Consistent**: Writes may not be immediately visible to all readers

### Implications for Access Modes

- **RWX (ReadWriteMany)**: ✅ Supported - Multiple pods can mount read-write
- **ROX (ReadOnlyMany)**: ✅ Supported - Multiple pods can mount read-only
- **RWO (ReadWriteOnce)**: ✅ Supported - Single pod mounts read-write

**Note:** While S3 supports concurrent writes (RWX), applications must handle:
- No file locking
- Eventual consistency
- Potential conflicts from concurrent writes

---

## Summary

### How ROX/RWX PVs Are Created

1. ✅ User creates PVC with `accessModes: [ReadOnlyMany]`
2. ✅ External-provisioner calls driver's `CreateVolume()`
3. ✅ Driver creates S3 bucket and returns volume info
4. ✅ External-provisioner creates PV with `accessModes: [ReadOnlyMany]`
5. ❌ Driver doesn't declare ROX support in capabilities
6. ❌ Driver doesn't handle ROX in `NodePublishVolume`
7. ❌ Volume is mounted read-write instead of read-only

### The Fix

1. Declare `MULTI_NODE_READER_ONLY` and `MULTI_NODE_MULTI_WRITER` in capabilities
2. Check access mode in `NodePublishVolume`
3. Set `ro` flag for ROX mounts
4. Pass `ro` flag to mounter
5. Mount with `-o ro` for read-only access

### Result

- ✅ PV access mode matches actual mount behavior
- ✅ ROX volumes are truly read-only
- ✅ RWX volumes remain read-write
- ✅ Driver properly supports all access modes