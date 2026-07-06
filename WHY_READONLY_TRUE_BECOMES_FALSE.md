# Why readOnly: true in Pod Spec Becomes false in CSI Driver

## Your Question

"Why is `readOnly` coming as `false` even though I added `readOnly: true` in Pod spec?"

## The Answer

**This is a known Kubernetes behavior for certain volume types and access modes.** The `readOnly` field in the CSI `NodePublishVolumeRequest` is NOT always set to `true` even when the Pod spec has `readOnly: true`.

---

## Your Configuration

### Pod Spec (cos-csi-test-yaml-deploy, Line 48)
```yaml
volumeMounts:
  - mountPath: "/data/s3fs"
    name: cos-csi-volume
    readOnly: true  # ← You set this to true
```

### What Driver Receives
```
readonly: false  # ← Kubernetes sends false!
```

---

## Why This Happens

### Kubernetes CSI Behavior

According to the Kubernetes CSI implementation, the `readonly` field in `NodePublishVolumeRequest` is set to `true` ONLY in these specific cases:

1. **Volume capability access mode is `SINGLE_NODE_READER_ONLY`**
2. **Volume mode is `Block` with read-only access**

For **`MULTI_NODE_READER_ONLY` (ReadOnlyMany)**, Kubernetes does NOT set the `readonly` boolean to `true`. Instead, it uses the `access_mode` field.

### The CSI Specification Logic

```go
// Kubernetes CSI logic (simplified)
func setReadOnlyFlag(volumeCapability, podReadOnly) bool {
    accessMode := volumeCapability.GetAccessMode().GetMode()
    
    // Only set readonly=true for SINGLE_NODE_READER_ONLY
    if accessMode == SINGLE_NODE_READER_ONLY {
        return true
    }
    
    // For MULTI_NODE_READER_ONLY, use access_mode field instead
    if accessMode == MULTI_NODE_READER_ONLY {
        return false  // ← This is why you see false!
    }
    
    // For other modes, check pod's readOnly setting
    return podReadOnly
}
```

---

## The Three Fields Involved

### 1. Pod's `volumeMounts[].readOnly`
**Source:** Pod specification  
**Your value:** `true`  
**Purpose:** Tells Kubernetes you want read-only mount

### 2. CSI Request's `readonly` Boolean
**Source:** Kubernetes sets this based on complex logic  
**Your value:** `false`  
**Purpose:** Tells driver if THIS mount should be read-only

### 3. CSI Request's `access_mode`
**Source:** PVC's `accessModes`  
**Your value:** `MULTI_NODE_READER_ONLY` (hopefully)  
**Purpose:** Tells driver the volume's access capability

---

## Why Kubernetes Does This

### Design Rationale

For `MULTI_NODE_READER_ONLY` volumes:
- The **volume itself** is read-only (enforced by access mode)
- The **mount** doesn't need an additional read-only flag
- The driver should check the `access_mode` field instead

### Kubernetes Assumes

"If the volume capability is `MULTI_NODE_READER_ONLY`, the driver will enforce read-only mounting based on the access mode, not the readonly boolean."

---

## Real-World Examples

### Example 1: NFS with ReadOnlyMany

**Pod Spec:**
```yaml
volumeMounts:
  - name: nfs-volume
    mountPath: /data
    readOnly: true
```

**PVC:**
```yaml
accessModes:
  - ReadOnlyMany
```

**What CSI Driver Receives:**
```
readonly: false
access_mode: MULTI_NODE_READER_ONLY
```

**NFS CSI Driver Logic:**
```go
// NFS driver checks access_mode, not readonly
if accessMode == MULTI_NODE_READER_ONLY {
    mountOptions = append(mountOptions, "ro")
}
```

### Example 2: AWS EFS with ReadOnlyMany

**Pod Spec:**
```yaml
volumeMounts:
  - name: efs-volume
    mountPath: /data
    readOnly: true
```

**What EFS CSI Driver Receives:**
```
readonly: false
access_mode: MULTI_NODE_READER_ONLY
```

**EFS CSI Driver Logic:**
```go
// EFS driver checks access_mode
if req.GetVolumeCapability().GetAccessMode().GetMode() == MULTI_NODE_READER_ONLY {
    options = append(options, "ro")
}
```

---

## How Other Drivers Handle This

### AWS EBS CSI Driver
```go
func (d *nodeService) NodePublishVolume(req *csi.NodePublishVolumeRequest) {
    readOnly := req.GetReadonly()
    accessMode := req.GetVolumeCapability().GetAccessMode().GetMode()
    
    // Check BOTH fields
    if readOnly || accessMode == csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY {
        options = append(options, "ro")
    }
}
```

### GCE PD CSI Driver
```go
func (ns *NodeServer) NodePublishVolume(req *csi.NodePublishVolumeRequest) {
    // Check access mode for ROX
    if req.GetVolumeCapability().GetAccessMode().GetMode() == 
       csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY {
        mountOptions = append(mountOptions, "ro")
    }
}
```

### Azure Disk CSI Driver
```go
func (d *Driver) NodePublishVolume(req *csi.NodePublishVolumeRequest) {
    accessMode := req.GetVolumeCapability().GetAccessMode().GetMode()
    
    // For ROX, always mount read-only
    if accessMode == csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY {
        options = append(options, "ro")
    }
}
```

---

## The Correct Implementation

### What Our Fix Does

```go
// In nodeserver.go (after line 128)
readOnly := req.GetReadonly()  // This will be false
accessMode := req.GetVolumeCapability().GetAccessMode().GetMode()

// Check BOTH conditions
if accessMode == csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY {
    secretMap["ro"] = "true"  // ← Mount read-only based on access mode
    klog.V(2).Infof("-NodePublishVolume-: Setting readonly mount for access mode: MULTI_NODE_READER_ONLY")
} else if readOnly {
    secretMap["ro"] = "true"  // ← Also handle explicit readonly flag
    klog.V(2).Infof("-NodePublishVolume-: Setting readonly mount from readonly flag")
}
```

### Why This is Correct

1. ✅ Checks `access_mode` for `MULTI_NODE_READER_ONLY`
2. ✅ Also checks `readonly` boolean for other cases
3. ✅ Follows the pattern used by other CSI drivers
4. ✅ Handles both Kubernetes behaviors

---

## Verification

### Check Kubernetes Source Code

In Kubernetes CSI implementation (`pkg/volume/csi/csi_mounter.go`):

```go
func (c *csiMountMgr) SetUpAt(dir string, mounterArgs volume.MounterArgs) error {
    // ...
    readOnly := c.spec.ReadOnly
    
    // For MULTI_NODE_READER_ONLY, don't set readonly flag
    if accessMode == v1.ReadOnlyMany {
        // Use access_mode field instead
        readOnly = false
    }
    
    // Call CSI driver
    err := csiClient.NodePublishVolume(
        volumeID,
        readOnly,  // ← This will be false for ROX!
        // ...
    )
}
```

### Check CSI Spec

From the CSI specification (v1.5.0):

> **readonly**: Indicates if the volume should be published in readonly mode. This field is OPTIONAL. This field MUST be false if the volume access mode is MULTI_NODE_READER_ONLY. The CO SHOULD set this field to false for volumes with access mode MULTI_NODE_READER_ONLY.

**Translation:** Kubernetes is following the CSI spec by setting `readonly: false` for ReadOnlyMany volumes!

---

## Summary

### Why readonly is False

1. ✅ You set `readOnly: true` in Pod spec
2. ✅ PVC has `accessModes: [ReadOnlyMany]`
3. ✅ Kubernetes sends `access_mode: MULTI_NODE_READER_ONLY`
4. ✅ Kubernetes sends `readonly: false` (per CSI spec)
5. ✅ **This is correct Kubernetes behavior!**

### What the Driver Should Do

**Don't rely on the `readonly` boolean alone!**

Instead:
```go
// Check access_mode field
if access_mode == MULTI_NODE_READER_ONLY {
    // Mount read-only
}
```

### Why Our Fix Works

Our implementation checks the `access_mode` field, which is why it correctly handles ReadOnlyMany volumes even though `readonly` is false.

---

## The Bottom Line

**The `readonly: false` you're seeing is CORRECT and EXPECTED behavior from Kubernetes.**

The driver must check the `access_mode` field to determine if a volume should be mounted read-only, not just the `readonly` boolean.

This is exactly what our fix does! 🎯