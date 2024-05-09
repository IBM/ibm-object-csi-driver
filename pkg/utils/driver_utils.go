package utils

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
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
	CheckMount(targetPath string) (bool, error)
	GetBucketUsage(volumeID string) (int64, resource.Quantity, error)
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

func (su *DriverStatsUtils) CheckMount(targetPath string) (bool, error) {
	out, err := exec.Command("mountpoint", targetPath).CombinedOutput()
	outStr := strings.TrimSpace(string(out))
	notMnt := true
	if err != nil {
		klog.V(3).Infof("Output: Output string error %+v", outStr)
		if strings.HasSuffix(outStr, "No such file or directory") {
			if err = os.MkdirAll(targetPath, 0750); err != nil {
				klog.V(2).Infof("checkMount: Error: %+v", err)
				return false, err
			}
			notMnt = true
		} else {
			return false, err
		}
	}
	return notMnt, nil
}

func (su *DriverStatsUtils) GetBucketUsage(volumeID string) (int64, resource.Quantity, error) {
	k8sClient, err := createK8sClient()
	if err != nil {
		return 0, resource.Quantity{}, err
	}

	secert, capacity, err := fetchSecret(k8sClient, volumeID)
	if err != nil {
		return 0, resource.Quantity{}, err
	}

	usage, err := bucketSizeUsed(secert)
	if err != nil {
		return 0, resource.Quantity{}, err
	}

	return usage, capacity, nil
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

func fetchSecret(clientset *kubernetes.Clientset, volumeID string) (*v1.Secret, resource.Quantity, error) {
	pvcName, pvcNamespace, capacity, err := getPVCNameFromPVID(clientset, volumeID)
	if err != nil {
		return nil, resource.Quantity{}, err
	}
	klog.Info("pvc details found. pvc-name: ", pvcName, ", pvc-namespace: ", pvcNamespace)

	secret, err := clientset.CoreV1().Secrets(pvcNamespace).Get(context.TODO(), pvcName, metav1.GetOptions{})
	if err != nil {
		return nil, resource.Quantity{}, fmt.Errorf("error getting Secret: %v", err)
	}

	if secret == nil {
		return nil, resource.Quantity{}, fmt.Errorf("secret not found with name: %v", pvcName)
	}

	klog.Info("secret details found. secret-name: ", secret.Name)
	return secret, capacity, nil
}

func getPVCNameFromPVID(clientset *kubernetes.Clientset, volumeID string) (string, string, resource.Quantity, error) {
	pv, err := clientset.CoreV1().PersistentVolumes().Get(context.TODO(), volumeID, metav1.GetOptions{})
	if err != nil {
		return "", "", resource.Quantity{}, fmt.Errorf("error getting PV: %v", err)
	}

	pvcName := pv.Spec.ClaimRef.Name
	if pvcName == "" {
		return "", "", resource.Quantity{}, fmt.Errorf("PVC name not found for PV with ID: %s", volumeID)
	}

	pvcNamespace := pv.Spec.ClaimRef.Namespace
	if pvcNamespace == "" {
		pvcNamespace = "default"
	}

	capacity := pv.Spec.Capacity["storage"]

	return pvcName, pvcNamespace, capacity, nil
}

func getDataFromSecret(secret *v1.Secret, key string) string {
	secretData := string(secret.Data[key])
	return secretData
}

func bucketSizeUsed(secret *v1.Secret) (int64, error) {
	locationConstraint := getDataFromSecret(secret, "locationConstraint")
	accessKey := getDataFromSecret(secret, "accessKey")
	secretKey := getDataFromSecret(secret, "secretKey")
	endpoint := getDataFromSecret(secret, "cosEndpoint")
	bucketName := getDataFromSecret(secret, "bucketName")

	// AWS Service configuration
	awsConfig := &aws.Config{
		Region:      aws.String(locationConstraint),
		Credentials: credentials.NewStaticCredentials(accessKey, secretKey, ""),
		Endpoint:    aws.String(endpoint),
	}

	// Initialize a new AWS session
	session, err := session.NewSession(awsConfig)
	if err != nil {
		klog.Error("Failed to initialize aws session")
		return 0, err
	}

	// Create an S3 client
	client := s3.New(session)

	// List objects in a bucket
	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(bucketName),
	}

	data, err := client.ListObjectsV2(input)
	if err != nil {
		klog.Error("Failed to list objects present in bucket")
		return 0, err
	}

	// Get summary
	var usage int64
	for _, item := range data.Contents {
		usage += *item.Size
	}

	return usage, nil
}
