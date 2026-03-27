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
	"google.golang.org/protobuf/proto"
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
	GetClusterNodeData(nodeName string) (*ClusterNodeData, error)
	GetEndpoints() (string, string, error)
	GetPVAttributes(volumeID string) (map[string]string, error)
	GetPVC(pvcName, pvcNamespace string) (*v1.PersistentVolumeClaim, error)
	GetSecret(secretName, secretNamespace string) (*v1.Secret, error)
	GetPV(volumeID string) (*v1.PersistentVolume, error)
}

type DriverStatsUtils struct {
}

type ClusterNodeData struct {
	Region string
	Zone   string
	OS     string
}

func (su *DriverStatsUtils) GetClusterNodeData(nodeName string) (*ClusterNodeData, error) {
	node, err := getNodeByName(nodeName)
	if err != nil {
		return nil, err
	}

	nodeLabels := node.Labels
	region, regionExists := nodeLabels[constants.NodeRegionLabel]
	zone, zoneExists := nodeLabels[constants.NodeZoneLabel]

	if !regionExists || !zoneExists {
		errorMsg := fmt.Errorf("one or few required node label(s) is/are missing [%s, %s]. Node Labels Found = [#%v]", constants.NodeRegionLabel, constants.NodeZoneLabel, nodeLabels) //nolint:golint
		return nil, errorMsg
	}

	data := &ClusterNodeData{
		Region: region,
		Zone:   zone,
		OS:     node.Status.NodeInfo.OSImage,
	}

	return data, nil
}

// GetEndpoints return IAMEndpoint, COSResourceConfigEndpoint, error
func (su *DriverStatsUtils) GetEndpoints() (string, string, error) {
	clusterType, err := getClusterType()
	if err != nil {
		return "", "", err
	}

	if strings.Contains(strings.ToLower(clusterType), "vpc") {
		// Use private iam endpoint for VPC clusters
		return constants.PrivateIAMEndpoint, constants.ResourceConfigEPDirect, nil
	}
	// Use public iam endpoint for classic clusters
	return constants.PublicIAMEndpoint, constants.ResourceConfigEPPrivate, nil
}

func (su *DriverStatsUtils) BucketToDelete(volumeID string) (string, error) {
	clientset, err := CreateK8sClient()
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
		klog.V(3).Infof("Check if mountPath exists: Output string- %+v", outStr)
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
	pv, err := su.GetPV(volumeID)
	if err != nil {
		return resource.Quantity{}, err
	}

	capacity := pv.Spec.Capacity["storage"]
	return capacity, nil
}

func (su *DriverStatsUtils) GetBucketUsage(volumeID string) (int64, error) {
	_, ep, err := su.GetEndpoints()
	if err != nil {
		return 0, err
	}

	secret, err := fetchSecretUsingPV(volumeID, su)
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
	pv, err := su.GetPV(volumeID)
	if err != nil {
		return "", err
	}

	tempBucketName := pv.Spec.CSI.VolumeAttributes["bucketName"]
	return tempBucketName, nil
}

func (su *DriverStatsUtils) GetPVAttributes(volumeID string) (map[string]string, error) {
	pv, err := su.GetPV(volumeID)
	if err != nil {
		return nil, err
	}

	return pv.Spec.CSI.VolumeAttributes, nil
}

func (su *DriverStatsUtils) GetPVC(pvcName, pvcNamespace string) (*v1.PersistentVolumeClaim, error) {
	k8sClient, err := CreateK8sClient()
	if err != nil {
		return nil, err
	}

	pvc, err := k8sClient.CoreV1().PersistentVolumeClaims(pvcNamespace).Get(context.TODO(), pvcName, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("Unable to fetch pvc %v", err)
		return nil, fmt.Errorf("error getting PVC: %v", err)
	}

	return pvc, nil
}

func (su *DriverStatsUtils) GetSecret(secretName, secretNamespace string) (*v1.Secret, error) {
	k8sClient, err := CreateK8sClient()
	if err != nil {
		return nil, err
	}

	secret, err := k8sClient.CoreV1().Secrets(secretNamespace).Get(context.TODO(), secretName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("error getting Secret: %v", err)
	}

	return secret, nil
}

func (su *DriverStatsUtils) GetPV(volumeID string) (*v1.PersistentVolume, error) {
	k8sClient, err := CreateK8sClient()
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

func ReplaceAndReturnCopy(req interface{}) (interface{}, error) {
	switch r := req.(type) {
	case *csi.CreateVolumeRequest:
		// Create a new CreateVolumeRequest and copy the original values
		var inReq *csi.CreateVolumeRequest

		newReq := proto.Clone(r).(*csi.CreateVolumeRequest)

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

		newReq := proto.Clone(r).(*csi.DeleteVolumeRequest)

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

		newReq := proto.Clone(r).(*csi.NodePublishVolumeRequest)

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

func CreateK8sClient() (*kubernetes.Clientset, error) {
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

func getClusterType() (string, error) {
	k8sClient, err := CreateK8sClient()
	if err != nil {
		return "", err
	}

	configMap, err := k8sClient.CoreV1().ConfigMaps("kube-system").Get(context.TODO(), "cluster-info", metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("error getting ConfigMap: %v", err)
	}

	clusterConfigStr := configMap.Data["cluster-config.json"]

	var clusterConfig map[string]string
	if err = json.Unmarshal([]byte(clusterConfigStr), &clusterConfig); err != nil {
		return "", fmt.Errorf("error unmarshalling cluster config: %v", err)
	}

	clusterType := clusterConfig["cluster_type"]
	klog.Info("Cluster Type ", clusterType)
	return clusterType, nil
}

func fetchSecretUsingPV(volumeID string, su *DriverStatsUtils) (*v1.Secret, error) {
	pv, err := su.GetPV(volumeID)
	if err != nil {
		return nil, err
	}
	klog.Info("secret fetched from PV:\n\t", pv.Spec.CSI.NodePublishSecretRef)

	secretName := pv.Spec.CSI.NodePublishSecretRef.Name
	secretNamespace := pv.Spec.CSI.NodePublishSecretRef.Namespace

	if secretName == "" {
		return nil, fmt.Errorf("secret details not found in the PV, could not fetch the secret")
	}

	if secretNamespace == "" {
		klog.Info("secret Namespace not found. trying to fetch the secret in default namespace")
		secretNamespace = constants.DefaultNamespace
	}

	secret, err := su.GetSecret(secretName, secretNamespace)
	if err != nil {
		return nil, fmt.Errorf("error getting Secret: %v", err)
	}

	if secret == nil {
		return nil, fmt.Errorf("secret not found with name: %v", secretNamespace)
	}

	klog.Info("secret details found. secretName: ", secret.Name)
	return secret, nil
}

func getNodeByName(nodeName string) (*v1.Node, error) {
	clientset, err := CreateK8sClient()
	if err != nil {
		return nil, err
	}

	node, err := clientset.CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return node, nil
}
