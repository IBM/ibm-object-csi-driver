# S3FS Command Examples: ReadOnlyMany vs ReadWriteMany

## Overview
This document shows the actual s3fs mount commands that will be executed for different access modes.

---

## Command Structure

### Basic s3fs Command Format
```bash
s3fs <bucket>[:<path>] <mountpoint> -o <options>
```

---

## Example 1: ReadWriteMany (RWX) - Default Behavior

### PVC Configuration
```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: balraj-cos-pvc
spec:
  accessModes:
    - ReadWriteMany  # RWX
  storageClassName: ibm-object-storage-standard-s3fs
  resources:
    requests:
      storage: 10Gi
```

### Resulting s3fs Command (WITHOUT -o ro)
```bash
s3fs ba-test-bucket /var/data/kubelet/pods/3491c6a3-0e47-447b-806b-21d0297ede7b/volumes/kubernetes.io~csi/pvc-c3423461-1167-46ca-81f0-7b7d6ce67f92/mount \
  -o url=https://s3.direct.us-south.cloud-object-storage.appdomain.cloud \
  -o endpoint=us-south \
  -o ibm_iam_auth \
  -o passwd_file=/etc/cos-s3-csi-driver/passwd-ba-test-bucket \
  -o multipart_size=52 \
  -o multireq_max=20 \
  -o max_dirty_data=5120 \
  -o parallel_count=20 \
  -o max_stat_cache_size=100000 \
  -o retries=5 \
  -o kernel_cache \
  -o allow_other \
  -o uid=0 \
  -o gid=0
```

**Key Point:** No `-o ro` flag - volume is mounted **read-write**

---

## Example 2: ReadOnlyMany (ROX) - With Our Fix

### PVC Configuration
```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: balraj-cos-pvc-2
spec:
  accessModes:
    - ReadOnlyMany  # ROX
  storageClassName: ibm-object-storage-standard-s3fs
  resources:
    requests:
      storage: 10Gi
```

### Resulting s3fs Command (WITH -o ro)
```bash
s3fs ba-test-bucket /var/data/kubelet/pods/0968c5c6-f0f6-4e06-88e6-6ca5d5370344/volumes/kubernetes.io~csi/pvc-25ebb694-d91d-4621-8142-f2623a50e8e5/mount \
  -o url=https://s3.direct.us-south.cloud-object-storage.appdomain.cloud \
  -o endpoint=us-south \
  -o ibm_iam_auth \
  -o passwd_file=/etc/cos-s3-csi-driver/passwd-ba-test-bucket \
  -o multipart_size=52 \
  -o multireq_max=20 \
  -o max_dirty_data=5120 \
  -o parallel_count=20 \
  -o max_stat_cache_size=100000 \
  -o retries=5 \
  -o kernel_cache \
  -o allow_other \
  -o uid=0 \
  -o gid=0 \
  -o ro
```

**Key Point:** Added `-o ro` flag - volume is mounted **read-only**

---

## Example 3: Pod with readOnly: true

### Pod Configuration
```yaml
apiVersion: v1
kind: Pod
metadata:
  name: test-pod
spec:
  containers:
  - name: app
    image: busybox
    volumeMounts:
    - name: data
      mountPath: /data
      readOnly: true  # Explicit readonly mount
  volumes:
  - name: data
    persistentVolumeClaim:
      claimName: balraj-cos-pvc  # Even if PVC is RWX
```

### Resulting s3fs Command (WITH -o ro)
```bash
s3fs ba-test-bucket /var/data/kubelet/pods/abc123.../mount \
  -o url=https://s3.direct.us-south.cloud-object-storage.appdomain.cloud \
  -o endpoint=us-south \
  -o ibm_iam_auth \
  -o passwd_file=/etc/cos-s3-csi-driver/passwd-ba-test-bucket \
  -o multipart_size=52 \
  -o multireq_max=20 \
  -o max_dirty_data=5120 \
  -o parallel_count=20 \
  -o max_stat_cache_size=100000 \
  -o retries=5 \
  -o kernel_cache \
  -o allow_other \
  -o uid=0 \
  -o gid=0 \
  -o ro
```

**Key Point:** `-o ro` flag added because Pod specified `readOnly: true`

---

## Comparison Table

| Scenario | PVC Access Mode | Pod readOnly | s3fs Command Includes `-o ro`? | Mount Behavior |
|----------|----------------|--------------|-------------------------------|----------------|
| Example 1 | ReadWriteMany | false | ❌ NO | Read-Write |
| Example 2 | ReadOnlyMany | false | ✅ YES | Read-Only |
| Example 3 | ReadWriteMany | true | ✅ YES | Read-Only |
| Example 4 | ReadOnlyMany | true | ✅ YES | Read-Only |

---

## Mount Options Breakdown

### Common Options (All Scenarios)
```bash
-o url=<S3_ENDPOINT>              # S3 service endpoint
-o endpoint=<REGION>              # AWS region or location
-o ibm_iam_auth                   # Use IBM IAM authentication
-o passwd_file=<PATH>             # Credentials file path
-o multipart_size=52              # Multipart upload size (MB)
-o multireq_max=20                # Max parallel requests
-o max_dirty_data=5120            # Max dirty data cache (MB)
-o parallel_count=20              # Parallel upload threads
-o max_stat_cache_size=100000     # Stat cache entries
-o retries=5                      # Retry count for operations
-o kernel_cache                   # Enable kernel page cache
-o allow_other                    # Allow other users to access
-o uid=0                          # User ID for files
-o gid=0                          # Group ID for files
```

### ReadOnly Option (ROX Only)
```bash
-o ro                             # Mount read-only (NEW!)
```

---

## How to Verify the Command

### Method 1: Check Driver Logs
```bash
# Get driver pod name
kubectl get pods -n kube-system | grep cos-s3-csi

# Check logs for mount command
kubectl logs -n kube-system <driver-pod-name> | grep "s3fs"
```

### Method 2: Check Process in Pod
```bash
# Exec into the pod
kubectl exec -it <pod-name> -- sh

# Check mount command
mount | grep s3fs

# Or check process
ps aux | grep s3fs
```

**Example Output for ROX:**
```
s3fs on /data type fuse.s3fs (ro,nosuid,nodev,relatime,user_id=0,group_id=0,allow_other)
```
Note the `ro` flag in the mount options!

### Method 3: Try to Write (Should Fail for ROX)
```bash
kubectl exec <pod-name> -- touch /data/test.txt
```

**Expected for ROX:**
```
touch: /data/test.txt: Read-only file system
```

**Expected for RWX:**
```
(Success - file created)
```

---

## Real-World Example from Your Log

### From readonlyMany-2.log (Line 3172)

**PVC:** `balraj-cos-pvc-2`  
**Access Mode:** `MULTI_NODE_READER_ONLY`  
**Bucket:** `ba-test-bucket`  
**Endpoint:** `https://s3.direct.us-south.cloud-object-storage.appdomain.cloud`

### Before Fix (Current Behavior)
```bash
s3fs ba-test-bucket /var/data/kubelet/pods/.../mount \
  -o url=https://s3.direct.us-south.cloud-object-storage.appdomain.cloud \
  -o endpoint=us-south \
  -o ibm_iam_auth \
  -o passwd_file=/etc/cos-s3-csi-driver/passwd-ba-test-bucket \
  -o multipart_size=52 \
  -o multireq_max=20 \
  -o max_dirty_data=5120 \
  -o parallel_count=20 \
  -o max_stat_cache_size=100000 \
  -o retries=5 \
  -o kernel_cache
```
❌ **Missing `-o ro` flag** - Mounted read-write (WRONG!)

### After Fix (Expected Behavior)
```bash
s3fs ba-test-bucket /var/data/kubelet/pods/.../mount \
  -o url=https://s3.direct.us-south.cloud-object-storage.appdomain.cloud \
  -o endpoint=us-south \
  -o ibm_iam_auth \
  -o passwd_file=/etc/cos-s3-csi-driver/passwd-ba-test-bucket \
  -o multipart_size=52 \
  -o multireq_max=20 \
  -o max_dirty_data=5120 \
  -o parallel_count=20 \
  -o max_stat_cache_size=100000 \
  -o retries=5 \
  -o kernel_cache \
  -o ro
```
✅ **Includes `-o ro` flag** - Mounted read-only (CORRECT!)

---

## Testing the Fix

### Step 1: Deploy Updated Driver
```bash
make build
# Deploy to cluster
```

### Step 2: Create ROX PVC
```bash
kubectl apply -f - <<EOF
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: test-rox-pvc
spec:
  accessModes:
    - ReadOnlyMany
  storageClassName: ibm-object-storage-standard-s3fs
  resources:
    requests:
      storage: 10Gi
EOF
```

### Step 3: Create Test Pod
```bash
kubectl apply -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: test-rox-pod
spec:
  containers:
  - name: test
    image: busybox
    command: ["sleep", "3600"]
    volumeMounts:
    - name: data
      mountPath: /data
  volumes:
  - name: data
    persistentVolumeClaim:
      claimName: test-rox-pvc
EOF
```

### Step 4: Verify Mount Command
```bash
# Check driver logs
kubectl logs -n kube-system <driver-pod> | grep "s3fs.*ro"

# Check mount in pod
kubectl exec test-rox-pod -- mount | grep s3fs

# Try to write (should fail)
kubectl exec test-rox-pod -- touch /data/test.txt
# Expected: Read-only file system error
```

---

## Summary

### Key Difference: The `-o ro` Flag

**Without Fix (RWX and ROX both):**
```bash
s3fs bucket /mount -o <options>
```

**With Fix (ROX only):**
```bash
s3fs bucket /mount -o <options> -o ro
```

### Impact

| Access Mode | Command | Write Test | Correct? |
|-------------|---------|------------|----------|
| RWX (before) | No `-o ro` | ✅ Success | ✅ Correct |
| RWX (after) | No `-o ro` | ✅ Success | ✅ Correct |
| ROX (before) | No `-o ro` | ✅ Success | ❌ WRONG! |
| ROX (after) | **With `-o ro`** | ❌ Fails | ✅ Correct |

The fix ensures that ReadOnlyMany volumes are truly read-only by adding the `-o ro` flag to the s3fs mount command.