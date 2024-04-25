package testsuites

import (
	"fmt"

	v2 "github.com/onsi/ginkgo/v2"
	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	clientset "k8s.io/client-go/kubernetes"
)

type PodDetails struct {
	Cmd      string
	CmdExits bool
	Volumes  []VolumeDetails
}

type VolumeDetails struct {
	PVCName     string //PVC Name
	VolumeType  string //PVC SC
	VolumeMount VolumeMountDetails
	pvc         *TestPersistentVolumeClaim
	ClaimSize   string //PVC Capacity
	AccessMode  *v1.PersistentVolumeAccessMode
}

type VolumeMountDetails struct {
	NameGenerate      string
	MountPathGenerate string
	ReadOnly          bool
}
type PodExecCheck struct {
	Cmd              []string
	ExpectedString01 string
	ExpectedString02 string
}

func (pod *PodDetails) SetupWithDynamicVolumes(client clientset.Interface, namespace *v1.Namespace) (*TestPod, []func()) {
	cleanupFuncs := make([]func(), 0)

	v2.By("setting up POD")
	tpod := NewTestPod(client, namespace, pod.Cmd)
	v2.By("setting up the PVC for POD")
	for n, v := range pod.Volumes {
		tpvc, funcs := v.SetupDynamicPersistentVolumeClaim(client, namespace)
		cleanupFuncs = append(cleanupFuncs, funcs...)

		tpod.SetupVolume(tpvc.persistentVolumeClaim,
			fmt.Sprintf("%s%d", v.VolumeMount.NameGenerate, n+1),
			fmt.Sprintf("%s%d", v.VolumeMount.MountPathGenerate, n+1), v.VolumeMount.ReadOnly)
	}
	return tpod, cleanupFuncs
}

func (volume *VolumeDetails) SetupDynamicPersistentVolumeClaim(client clientset.Interface, namespace *v1.Namespace) (*TestPersistentVolumeClaim, []func()) {
	cleanupFuncs := make([]func(), 0)
	v2.By("setting up the PVC and PV")
	//By(fmt.Sprintf("PVC: %q    NS: %q", volume.PVCName, namespace.Name))
	storageClass := storagev1.StorageClass{}
	storageClass.Name = volume.VolumeType
	tpvc := NewTestPersistentVolumeClaim(client, volume.PVCName, namespace, volume.ClaimSize, volume.AccessMode, &storageClass)
	tpvc.Create()
	cleanupFuncs = append(cleanupFuncs, tpvc.Cleanup)
	// PV will not be ready until PVC is used in a pod when volumeBindingMode: WaitForFirstConsumer
	tpvc.WaitForBound()
	tpvc.ValidateProvisionedPersistentVolume()
	volume.pvc = tpvc

	return tpvc, cleanupFuncs
}
