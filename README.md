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

Deploy the resources as below based on managed and unmanaged clusters.

## For IBM Managed clusters 

Review `deploy/ibmCloud/kustomization.yaml` file.

Update images if required
```
- name: cos-driver-image
  newName: icr.io/ibm/ibm-object-csi-driver
  newTag: v0.1.11
```

Update IBM COS endpoint and locationconstraint as per the region of your cluster
```
value: "https://s3.direct.au-syd.cloud-object-storage.appdomain.cloud"
value: "au-syd-standard"
```

Deploy the driver
`kubectl apply -k deploy/ibmCloud/`


To clean up the deployment 

`kubectl delete -k deploy/ibmCloud/`

After deployment following storage classes will be available in the cluster 
```
NAME                                          PROVISIONER            RECLAIMPOLICY   VOLUMEBINDINGMODE      ALLOWVOLUMEEXPANSION   AGE
ibm-object-storage-smart-rclone               cos.s3.csi.ibm.io      Delete          Immediate              false
ibm-object-storage-smart-rclone-retain        cos.s3.csi.ibm.io      Retain          Immediate              false
ibm-object-storage-smart-s3fs                 cos.s3.csi.ibm.io      Delete          Immediate              false
ibm-object-storage-smart-s3fs-retain          cos.s3.csi.ibm.io      Retain          Immediate              false
ibm-object-storage-standard-rclone            cos.s3.csi.ibm.io      Delete          Immediate              false
ibm-object-storage-standard-rclone-retain     cos.s3.csi.ibm.io      Retain          Immediate              false
ibm-object-storage-standard-s3fs              cos.s3.csi.ibm.io      Delete          Immediate              false
ibm-object-storage-standard-s3fs-retain       cos.s3.csi.ibm.io      Retain          Immediate              false
```


## For unmanaged clusters

First review `deploy/ibmCloud/kustomization.yaml` file and update IBM COS endpoint and locationconstraint as per the region of your cluster
```
value: "https://s3.direct.au-syd.cloud-object-storage.appdomain.cloud"
value: "au-syd-standard"
```
Then review `deploy/ibmUnmanaged/kustomization.yaml` and update images if required
```
- name: cos-driver-image
  newName: quay.io/containerstorage/ibm-object-csi-driver
  newTag: v0.1.11
```

Deploy the driver
`kubectl apply -k deploy/ibmUnmanaged/`


To clean up the deployment

`kubectl delete -k deploy/ibmUnmanaged/`

# Testing

Provide proper values for parameters in secret under examples/kubernetes/cos-s3-csi-<mounter_type>-secret.yaml

1. Create Secret, PVC and POD 
   Pick the respective files based on the mounter you want to deploy. Below is for s3fs mounter.
      `kubectl create -f examples/kubernetes/cos-s3-csi-s3fs-secret.yaml`

      If you want to use your own bucket, bucketName should be specified in the secret. If left empty, a temp bucket will be generated.

      `kubectl create -f examples/kubernetes/cos-s3-csi-s3fs-pvc.yaml`

      `kubectl create -f examples/kubernetes/cos-csi-app-s3fs.yaml`
    
    For rclone mounter, if any other rclone mount options need to be provided, they can be passed in Secret using StringData field.
    For example -
    ```
    stringData: 
        mountOptions: |
            upload_concurrency=30
            low_level_retries=3
    ```

    For non-root user support, in the Secret  user can add `uid` which must match `RunAsUser` in Pod spec.
    Example -
    ```
    stringData:
      uid: "3000" # Provide uid to run as non root user. This must match runAsUser in SecurityContext of pod spec.
    ```
    User can skip changes in Secret and directly use Pod Spec to enforce non root volume mount by providing `RunAsUser` value same as `FsGroup`.

2. Verify PVC is in `Bound` state

3. Check for successful mount as below:

If mounter type is `rclone`, verify using command
   ```
   # mount | grep rclone
   rclone-remote:rcloneambfail on /data type fuse.rclone (rw,nosuid,nodev,relatime,user_id=0,group_id=0)

   ```
If mounter type is `s3fs`, verify using command


   ```
    # mount | grep s3fs
    s3fs on /data type fuse.s3fs (rw,nosuid,nodev,relatime,user_id=0,group_id=0,allow_other)

   ```

# Debug 

Collect logs using below commands to check failure messages

1.  `oc logs cos-s3-csi-controller-0 -c cos-csi-provisioner`
2.  `oc logs cos-s3-csi-driver-xxx -c cos-csi-driver`
