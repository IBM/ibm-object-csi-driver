## Doc
https://cloud.ibm.com/docs/cloud-object-storage?topic=cloud-object-storage-endpoints

## Export ENV 
```
export E2E_TEST_RESULT=<file-path-were-results-are-saved>
export KUBECONFIG=<kube-config path>
export cosEndpoint=https://s3.us-south.cloud-object-storage.appdomain.cloud
export locationConstraint=us-geo 
export bucketName=testbuckete2eone
export accessKey=xxx
export secretKey=yyy
```

## Run E2E
```
ginkgo -v -nodes=1 ./tests/e2e -- -e2e-verify-service-account=false
```

## Results

cat e2e-test.out (Path provided in E2E_TEST_RESULT)

```
OBJECT-CSI-PLUGIN(s3fs): PVC CREATE, POD MOUNT, READ/WRITE, CLEANUP : PASS
```

