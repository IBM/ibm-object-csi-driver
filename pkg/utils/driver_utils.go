package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/IBM/go-sdk-core/v5/core"
	rc "github.com/IBM/ibm-cos-sdk-go-config/v2/resourceconfigurationv1"
	"github.com/IBM/ibm-object-csi-driver/pkg/constants"
	"github.com/container-storage-interface/spec/lib/go/csi"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/volume/util/fs"
)

type StatsUtils interface {
	BucketToDelete(volumeID string) (string, error)
	FSInfo(path string) (int64, int64, int64, int64, int64, int64, error)
	CheckMount(targetPath string) error
	GetTotalCapacityFromPV(volumeID string) (resource.Quantity, error)
	GetBucketUsage(volumeID string) (int64, error)
	GetBucketNameFromPV(volumeID string) (string, error)
}

type DriverStatsUtils struct {
}

func (su *DriverStatsUtils) BucketToDelete(volumeID string) (string, error) {
	clientset, err := createK8sClient()
	if err != nil {
		return "", err
	}

	pv, err := clientset.CoreV1().PersistentVolumes().Get(context.Background(), volumeID, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("Unable to fetch bucket %v", err)
		return "", err
	}

	klog.Infof("***Attributes: %v", pv.Spec.CSI.VolumeAttributes)
	if pv.Spec.CSI.VolumeAttributes["userProvidedBucket"] != "true" {
		klog.Infof("Bucket will be deleted %v", pv.Spec.CSI.VolumeAttributes["bucketName"])
		return pv.Spec.CSI.VolumeAttributes["bucketName"], nil
	}

	klog.Infof("Bucket will be persisted %v", pv.Spec.CSI.VolumeAttributes["bucketName"])
	return "", nil
}

func (su *DriverStatsUtils) FSInfo(path string) (int64, int64, int64, int64, int64, int64, error) {
	return fs.Info(path)
}

func (su *DriverStatsUtils) CheckMount(targetPath string) error {
	out, err := exec.Command("mountpoint", targetPath).CombinedOutput()
	outStr := strings.TrimSpace(string(out))
	if err != nil {
		klog.V(3).Infof("Output: Output string error %+v", outStr)
		if strings.HasSuffix(outStr, "No such file or directory") {
			if err = os.MkdirAll(targetPath, 0750); err != nil {
				klog.V(2).Infof("checkMount: Error: %+v", err)
				return err
			}
		} else {
			return err
		}
	}
	return nil
}

func (su *DriverStatsUtils) GetTotalCapacityFromPV(volumeID string) (resource.Quantity, error) {
	pv, err := getPV(volumeID)
	if err != nil {
		return resource.Quantity{}, err
	}

	capacity := pv.Spec.Capacity["storage"]
	return capacity, nil
}

func (su *DriverStatsUtils) GetBucketUsage(volumeID string) (int64, error) {
	ep, err := getEPBasedOnCluserInfra()
	if err != nil {
		return 0, err
	}

	secret, err := fetchSecretUsingPV(volumeID)
	if err != nil {
		return 0, err
	}

	apiKey := string(secret.Data["apiKey"])
	bucketName := string(secret.Data["bucketName"])

	rcOptions := &rc.ResourceConfigurationV1Options{
		URL: ep,
		Authenticator: &core.IamAuthenticator{
			ApiKey: apiKey, // pragma: allowlist secret
			URL:    constants.IAMEP,
		},
	}
	resourceConfig, err := rc.NewResourceConfigurationV1(rcOptions)
	if err != nil {
		klog.Error("Failed to create resource config")
		return 0, err
	}

	bucketOptions := &rc.GetBucketConfigOptions{
		Bucket: &bucketName,
	}

	res, _, err := resourceConfig.GetBucketConfig(bucketOptions)
	if err != nil {
		klog.Error("Failed to get bucket config")
		return 0, err
	}

	return *res.BytesUsed, nil
}

func (su *DriverStatsUtils) GetBucketNameFromPV(volumeID string) (string, error) {
	pv, err := getPV(volumeID)
	if err != nil {
		return "", err
	}

	tempBucketName := pv.Spec.CSI.VolumeAttributes["bucketName"]
	return tempBucketName, nil
}

func ReplaceAndReturnCopy(req interface{}) (interface{}, error) {
	switch r := req.(type) {
	case *csi.CreateVolumeRequest:
		// Create a new CreateVolumeRequest and copy the original values
		var inReq *csi.CreateVolumeRequest

		newReq := &csi.CreateVolumeRequest{}
		*newReq = *r

		inReq = req.(*csi.CreateVolumeRequest)

		// Modify the Secrets map in the new request
		newReq.Secrets = make(map[string]string)
		secretMap := inReq.GetSecrets()

		for k, v := range secretMap {
			if k == "accessKey" || k == "secretKey" || k == "apiKey" || k == "kpRootKeyCRN" {
				newReq.Secrets[k] = "xxxxxxx"
				continue
			}
			newReq.Secrets[k] = v
		}
		return newReq, nil
	case *csi.DeleteVolumeRequest:
		// Create a new DeleteVolumeRequest and copy the original values
		var inReq *csi.DeleteVolumeRequest

		newReq := &csi.DeleteVolumeRequest{}
		*newReq = *r

		inReq = req.(*csi.DeleteVolumeRequest)

		// Modify the Secrets map in the new request
		newReq.Secrets = make(map[string]string)
		secretMap := inReq.GetSecrets()

		for k, v := range secretMap {
			if k == "accessKey" || k == "secretKey" || k == "apiKey" || k == "kpRootKeyCRN" {
				newReq.Secrets[k] = "xxxxxxx"
				continue
			}
			newReq.Secrets[k] = v
		}

		return newReq, nil
	case *csi.NodePublishVolumeRequest:
		// Create a new NodePublishVolumeRequest and copy the original values
		var inReq *csi.NodePublishVolumeRequest

		newReq := &csi.NodePublishVolumeRequest{}
		*newReq = *r

		inReq = req.(*csi.NodePublishVolumeRequest)

		// Modify the Secrets map in the new request
		newReq.Secrets = make(map[string]string)
		secretMap := inReq.GetSecrets()

		for k, v := range secretMap {
			if k == "accessKey" || k == "secretKey" || k == "apiKey" || k == "kpRootKeyCRN" {
				newReq.Secrets[k] = "xxxxxxx"
				continue
			}
			newReq.Secrets[k] = v
		}

		return newReq, nil

	default:
		return req, fmt.Errorf("unsupported request type")
	}
}

func createK8sClient() (*kubernetes.Clientset, error) {
	// Create a Kubernetes client configuration
	config, err := rest.InClusterConfig()
	if err != nil {
		klog.Error("Error creating Kubernetes client configuration: ", err)
		return nil, err
	}

	// Create a Kubernetes clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Error("Error creating Kubernetes clientset: ", err)
		return nil, err
	}

	return clientset, nil
}

func getPV(volumeID string) (*v1.PersistentVolume, error) {
	k8sClient, err := createK8sClient()
	if err != nil {
		return nil, err
	}

	pv, err := k8sClient.CoreV1().PersistentVolumes().Get(context.Background(), volumeID, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("Unable to fetch pv %v", err)
		return nil, fmt.Errorf("error getting PV: %v", err)
	}

	return pv, nil
}

func getSecret(pvcName, pvcNamespace string) (*v1.Secret, error) {
	k8sClient, err := createK8sClient()
	if err != nil {
		return nil, err
	}

	secret, err := k8sClient.CoreV1().Secrets(pvcNamespace).Get(context.TODO(), pvcName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("error getting Secret: %v", err)
	}

	return secret, nil
}

func getEPBasedOnCluserInfra() (string, error) {
	k8sClient, err := createK8sClient()
	if err != nil {
		return "", err
	}

	configMap, err := k8sClient.CoreV1().ConfigMaps("kube-system").Get(context.TODO(), "cluster-info", metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("error getting ConfigMap: %v", err)
	}

	clusterConfigStr := configMap.Data["cluster-config.json"]
	klog.Info("Successfully fetched Cluster Config ", clusterConfigStr)

	var clusterConfig map[string]string
	if err = json.Unmarshal([]byte(clusterConfigStr), &clusterConfig); err != nil {
		return "", fmt.Errorf("error unmarshalling cluster config: %v", err)
	}

	clusterType := clusterConfig["cluster_type"]
	klog.Info("Cluster Type ", clusterType)

	if strings.Contains(clusterType, "vpc") {
		return constants.ResourceConfigEPDirect, nil
	}
	return constants.ResourceConfigEPPrivate, nil
}

func fetchSecretUsingPV(volumeID string) (*v1.Secret, error) {
	pv, err := getPV(volumeID)
	if err != nil {
		return nil, err
	}

	pvcName := pv.Spec.ClaimRef.Name
	if pvcName == "" {
		return nil, fmt.Errorf("PVC name not found for PV with ID: %s", volumeID)
	}
	pvcNamespace := pv.Spec.ClaimRef.Namespace
	if pvcNamespace == "" {
		pvcNamespace = "default"
	}

	secret, err := getSecret(pvcName, pvcNamespace)
	if err != nil {
		return nil, fmt.Errorf("error getting Secret: %v", err)
	}

	if secret == nil {
		return nil, fmt.Errorf("secret not found with name: %v", pvcName)
	}

	klog.Info("secret details found. secret-name: ", secret.Name)
	return secret, nil
}
