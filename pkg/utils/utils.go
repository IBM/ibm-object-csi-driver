package utils

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/mitchellh/go-ps"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/volume/util/fs"
)

var unmount = syscall.Unmount

type StatsUtils interface {
	BucketToDelete(volumeID string) (string, error)
	FSInfo(path string) (int64, int64, int64, int64, int64, int64, error)
	CheckMount(targetPath string) (bool, error)
	FuseUnmount(path string) error
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

func (su *DriverStatsUtils) FuseUnmount(path string) error {
	// directory exists
	isMount, checkMountErr := isMountpoint(path)
	if isMount || checkMountErr != nil {
		klog.Infof("isMountpoint  %v", isMount)
		err := unmount(path, 0)
		if err != nil && checkMountErr == nil {
			klog.Errorf("Cannot unmount. Trying force unmount %s", err)
			//Do force unmount
			err = unmount(path, 0)
			if err != nil {
				klog.Errorf("Cannot force unmount %s", err)
				return fmt.Errorf("cannot force unmount %s: %v", path, err)
			}
		}
	}
	// as fuse quits immediately, we will try to wait until the process is done
	process, err := findFuseMountProcess(path)
	if err != nil {
		klog.Infof("Error getting PID of fuse mount: %s", err)
		return nil
	}
	if process == nil {
		klog.Infof("Unable to find PID of fuse mount %s, it must have finished already", path)
		return nil
	}
	klog.Infof("Found fuse pid %v of mount %s, checking if it still runs", process.Pid, path)
	return waitForProcess(process, 1)
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

func isMountpoint(pathname string) (bool, error) {
	klog.Infof("Checking if path is mountpoint: Pathname - %s", pathname)

	out, err := exec.Command("mountpoint", pathname).CombinedOutput()
	outStr := strings.TrimSpace(string(out))
	if err != nil {
		if strings.HasSuffix(outStr, "Transport endpoint is not connected") {
			return true, err
		}
		return false, err
	}

	if strings.HasSuffix(outStr, "is a mountpoint") {
		klog.Infof("Path is a mountpoint: pathname - %s", pathname)
		return true, nil
	} else if strings.HasSuffix(outStr, "is not a mountpoint") {
		klog.Infof("Path is NOT a mountpoint:Pathname - %s", pathname)
		return false, nil
	}
	klog.Errorf("Cannot parse mountpoint result: %v", outStr)
	return false, fmt.Errorf("cannot parse mountpoint result: %s", outStr)
}

func findFuseMountProcess(path string) (*os.Process, error) {
	processes, err := ps.Processes()
	if err != nil {
		return nil, err
	}
	for _, p := range processes {
		cmdLine, err := getCmdLine(p.Pid())
		if err != nil {
			klog.Errorf("Unable to get cmdline of PID %v: %s", p.Pid(), err)
			continue
		}
		if strings.Contains(cmdLine, path) {
			klog.Infof("Found matching pid %v on path %s", p.Pid(), path)
			return os.FindProcess(p.Pid())
		}
	}
	return nil, nil
}

func getCmdLine(pid int) (string, error) {
	cmdLineFile := fmt.Sprintf("/proc/%v/cmdline", pid)
	cmdLine, err := os.ReadFile(cmdLineFile) // #nosec G304: Dynamic pid .
	if err != nil {
		return "", err
	}
	return string(cmdLine), nil
}

func waitForProcess(p *os.Process, backoff int) error {
	if backoff == 20 {
		return fmt.Errorf("timeout waiting for PID %v to end", p.Pid)
	}
	cmdLine, err := getCmdLine(p.Pid)
	if err != nil {
		klog.Warningf("Error checking cmdline of PID %v, assuming it is dead: %s", p.Pid, err)
		return nil
	}
	if cmdLine == "" {
		// ignore defunct processes
		// TODO: debug why this happens in the first place
		// seems to only happen on k8s, not on local docker
		klog.Warning("Fuse process seems dead, returning")
		return nil
	}
	if err := p.Signal(syscall.Signal(0)); err != nil {
		klog.Warningf("Fuse process does not seem active or we are unprivileged: %s", err)
		return nil
	}
	klog.Infof("Fuse process with PID %v still active, waiting...", p.Pid)
	time.Sleep(time.Duration(backoff*100) * time.Millisecond)
	return waitForProcess(p, backoff+1)
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
