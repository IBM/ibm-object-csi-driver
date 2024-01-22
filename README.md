# ibm-object-csi-driver
CSI base Object Storage driver/plug-in. Currently, the driver supports s3fs and rclone mounters.

# Build the driver

For building the driver `docker` and `GO` should be installed on the system

1. On your local machine, install [`docker`](https://docs.docker.com/install/) and [`Go`](https://golang.org/doc/install).
2. Install latest Go 
3. Set the [`GOPATH` environment variable](https://github.com/golang/go/wiki/SettingGOPATH).
4. Build the driver image

   ## Clone the repo or your forked repo

   ```
   $ mkdir -p $GOPATH/src/github.com/IBM
   $ cd $GOPATH/src/github.com/IBM/
   $ git clone https://github.com/IBM/ibm-object-csi-driver.git
   $ cd ibm-object-csi-driver
   ```
   ## Build container image for the driver

   ```
   export RHSM_USER=<RHSM_USER>
   export RHSM_PASS=<RHSM_PASS>

   make container
   ```

An image named `ibm-object-csi-driver:latest` is created. Please retag and push the image to suitable registries to deploy in cluster.

# Deploy CSI driver on your cluster

Update images if required in `deploy/ibmCloud/kustomization.yaml` file. 

Deploy the resources

## For IBM Managed clusters 

`kubectl apply -k deploy/ibmCloud/`


To clean up the deployment 

`kubectl delete -k deploy/ibmCloud/`



## For unmanaged clusters

`kubectl apply -k deploy/ibmUnmanaged/`


To clean up the deployment

`kubectl delete -k deploy/ibmUnmanaged/`

# Testing

Provide proper values for parameters in secret under examples/cos-s3-csi-pvc-secret.yaml

1. Create Secret, PVC and POD 

      `kubectl create -f examples/cos-s3-csi-pvc-secret.yaml`

      If you want to use your own bucket, bucketName should be specified in the secret. If left empty, a temp bucket will be generated. Temp bucket will be deleted after volume is unpublished, unless bucketProtection value is set to Retain in the secret. If bucketProtection is set to Retain then, even if PersistentVolumeReclaimPolicy is Delete, the bucket will be retained for the user. 

      `kubectl create -f examples/cos-s3-csi-pvc.yaml`

      `kubectl create -f examples/cos-csi-app.yaml`
    
    If rclone mount options need to be provided they can be provided in Secret using StringData field.
    For example
    ```
    stringData: 
        mountOptions: |
            upload_concurrency=30
            low_level_retries=3
    ```

2. Verify PVC is in `Bound` state

3. Check for successful mount

If mounter type is `rclone`, you will see
   ```
   mount | grep rclone
   rclone-remote:rcloneambfail on /data type fuse.rclone (rw,nosuid,nodev,relatime,user_id=0,group_id=0)

   ```
If mounter type is `s3fs`, you will see


   ```
    root@cos-csi-app:/# mount | grep s3fs
    s3fs on /data type fuse.s3fs (rw,nosuid,nodev,relatime,user_id=0,group_id=0,allow_other)

   ```

# Debug 

Collect logs using below commands to check failure messages

1.  `oc logs cos-s3-csi-controller-0 -c cos-csi-provisioner`
2.  `oc logs cos-s3-csi-driver-xxx -c cos-csi-driver`
