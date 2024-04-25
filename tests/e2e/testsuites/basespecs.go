/**
 * Copyright 2023 IBM Corp.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package testsuites

import (
	"context"
	"fmt"
	"time"

	v2 "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	k8sDevPod "k8s.io/kubernetes/test/e2e/framework/pod"
	e2eoutput "k8s.io/kubernetes/test/e2e/framework/pod/output"
	k8sDevPV "k8s.io/kubernetes/test/e2e/framework/pv"
	imageutils "k8s.io/kubernetes/test/utils/image"
)

type TestSecret struct {
	client             clientset.Interface
	name               string
	namespace          string
	cosEndpoint        string
	locationConstraint string
	bucketName         string
	accessKey          string
	secretKey          string
	secType            string
	secret             *v1.Secret
}

type TestPersistentVolumeClaim struct {
	name                           string
	client                         clientset.Interface
	storageClass                   *storagev1.StorageClass
	namespace                      *v1.Namespace
	claimSize                      string
	accessMode                     v1.PersistentVolumeAccessMode
	persistentVolume               *v1.PersistentVolume
	persistentVolumeClaim          *v1.PersistentVolumeClaim
	requestedPersistentVolumeClaim *v1.PersistentVolumeClaim
}

type TestPod struct {
	client      clientset.Interface
	pod         *v1.Pod
	namespace   *v1.Namespace
	dumpDbgInfo bool
	dumpLog     bool
}

func NewTestPod(c clientset.Interface, ns *v1.Namespace, command string) *TestPod {
	return &TestPod{
		dumpDbgInfo: true,
		dumpLog:     true,
		client:      c,
		namespace:   ns,
		pod: &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "obj-e2e-tester-",
				Labels:       map[string]string{"app": "obj-vol-e2e"},
			},
			Spec: v1.PodSpec{
				Containers: []v1.Container{
					{
						Name:         "obj-e2e-tester",
						Image:        imageutils.GetE2EImage(imageutils.BusyBox),
						Command:      []string{"/bin/sh"},
						Args:         []string{"-c", command},
						VolumeMounts: make([]v1.VolumeMount, 0),
					},
				},
				RestartPolicy: v1.RestartPolicyNever,
				Volumes:       make([]v1.Volume, 0),
			},
		},
	}
}

func NewSecret(c clientset.Interface, name, stype, ns, cosEndpoint, locationConstraint, bucketName, accessKey, secretKey string) *TestSecret {
	return &TestSecret{
		client:             c,
		name:               name,
		namespace:          ns,
		cosEndpoint:        cosEndpoint,
		locationConstraint: locationConstraint,
		bucketName:         bucketName,
		accessKey:          accessKey,
		secretKey:          secretKey,
		secType:            stype,
	}
}

func (s *TestSecret) Create() {
	var err error
	v2.By("creating Secret")
	framework.Logf("creating Secret %q under %q", s.name, s.namespace)
	secret := v1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind: "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.name,
			Namespace: s.namespace,
		},
		Data: map[string][]byte{
			"cosEndpoint":        []byte(s.cosEndpoint),
			"locationConstraint": []byte(s.locationConstraint),
			"bucketName":         []byte(s.bucketName),
			"accessKey":          []byte(s.accessKey),
			"secretKey":          []byte(s.secretKey),
		},
		Type: v1.SecretType(s.secType),
	}
	s.secret, err = s.client.CoreV1().Secrets(s.namespace).Create(context.Background(), &secret, metav1.CreateOptions{})
	framework.ExpectNoError(err)
}

func (s *TestSecret) Cleanup() {
	v2.By("deleting Secret")
	framework.Logf("deleting Secret [%s]", s.secret.Name)
	err := s.client.CoreV1().Secrets(s.namespace).Delete(context.Background(), s.secret.Name, metav1.DeleteOptions{})
	framework.ExpectNoError(err)
}

func (t *TestPod) Create() {
	var err error

	t.pod, err = t.client.CoreV1().Pods(t.namespace.Name).Create(context.Background(), t.pod, metav1.CreateOptions{})
	framework.ExpectNoError(err)
}

func (t *TestPod) Delete() {
	v2.By("POD Delete: deleting POD")
	framework.Logf("deleting POD [%s/%s]", t.namespace.Name, t.pod.Name)
	e2eoutput.DumpDebugInfo(t.client, t.namespace.Name)
	err := t.client.CoreV1().Pods(t.namespace.Name).Delete(context.Background(), t.pod.Name, metav1.DeleteOptions{})
	framework.ExpectNoError(err)
}

func (t *TestPod) Exec(command []string, expectedString string) {
	v2.By("POD Exec: executing cmd in POD")
	framework.Logf("executing cmd in POD [%s/%s]", t.namespace.Name, t.pod.Name)
	_, err := e2eoutput.LookForStringInPodExec(t.namespace.Name, t.pod.Name, command, expectedString, execTimeout)
	framework.ExpectNoError(err)
}

func (t *TestPod) WaitForSuccess() {
	v2.By(fmt.Sprintf("checking that the pods command exits with no error [%s/%s]", t.namespace.Name, t.pod.Name))
	err := k8sDevPod.WaitForPodSuccessInNamespaceSlow(t.client, t.pod.Name, t.namespace.Name)
	framework.ExpectNoError(err)
}

var podRunningCondition = func(pod *v1.Pod) (bool, error) {
	switch pod.Status.Phase {
	case v1.PodRunning:
		v2.By("Saw pod running")
		return true, nil
	case v1.PodSucceeded:
		return true, fmt.Errorf("pod %q successed with reason: %q, message: %q", pod.Name, pod.Status.Reason, pod.Status.Message)
	default:
		return false, nil
	}
}

func (t *TestPod) WaitForRunningSlow() {
	v2.By(fmt.Sprintf("checking that the pods status is running [%s/%s]", t.namespace.Name, t.pod.Name))
	//err := framework.WaitTimeoutForPodRunningInNamespace(t.client, t.pod.Name, t.namespace.Name, slowPodStartTimeout)
	err := k8sDevPod.WaitForPodCondition(t.client, t.namespace.Name, t.pod.Name, failedConditionDescription, slowPodStartTimeout, podRunningCondition)
	framework.ExpectNoError(err)
}

func (t *TestPod) WaitForRunning() {
	err := k8sDevPod.WaitForPodRunningInNamespace(t.client, t.pod)
	framework.ExpectNoError(err)
}

func (t *TestPod) SetupVolume(pvc *v1.PersistentVolumeClaim, name, mountPath string, readOnly bool) {
	volumeMount := v1.VolumeMount{
		Name:      name,
		MountPath: mountPath,
		ReadOnly:  readOnly,
	}
	t.pod.Spec.Containers[0].VolumeMounts = append(t.pod.Spec.Containers[0].VolumeMounts, volumeMount)

	volume := v1.Volume{
		Name: name,
		VolumeSource: v1.VolumeSource{
			PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
				ClaimName: pvc.Name,
			},
		},
	}
	t.pod.Spec.Volumes = append(t.pod.Spec.Volumes, volume)
}

func NewTestPersistentVolumeClaim(
	c clientset.Interface, pvcName string, ns *v1.Namespace,
	claimSize string, accessmode *v1.PersistentVolumeAccessMode,
	sc *storagev1.StorageClass) *TestPersistentVolumeClaim {
	pvcAccessMode := v1.ReadWriteOnce
	if accessmode != nil {
		pvcAccessMode = *accessmode
	}

	return &TestPersistentVolumeClaim{
		name:         pvcName,
		client:       c,
		namespace:    ns,
		storageClass: sc,
		claimSize:    claimSize,
		accessMode:   pvcAccessMode,
	}
}

func (t *TestPersistentVolumeClaim) Create() {
	var err error

	v2.By("creating a PVC")
	storageClassName := ""
	if t.storageClass != nil {
		storageClassName = t.storageClass.Name
	}
	_, err = t.client.StorageV1().StorageClasses().Get(context.Background(), storageClassName, metav1.GetOptions{})
	framework.ExpectNoError(err)

	t.requestedPersistentVolumeClaim = generatePVC(t.name, t.namespace.Name, storageClassName, t.claimSize, t.accessMode)
	t.persistentVolumeClaim, err = t.client.CoreV1().PersistentVolumeClaims(t.namespace.Name).Create(context.Background(), t.requestedPersistentVolumeClaim, metav1.CreateOptions{})
	framework.ExpectNoError(err)
}

func (t *TestPersistentVolumeClaim) Cleanup() {
	v2.By(fmt.Sprintf("deleting PVC [%s]", t.persistentVolumeClaim.Name))
	err := k8sDevPV.DeletePersistentVolumeClaim(t.client, t.persistentVolumeClaim.Name, t.namespace.Name)
	v2.By("Triggered DeletePersistentVolumeClaim call")
	framework.ExpectNoError(err)
	// Wait for the PV to get deleted if reclaim policy is Delete. (If it's
	// Retain, there's no use waiting because the PV won't be auto-deleted and
	// it's expected for the caller to do it.) Technically, the first few delete
	// attempts may fail, as the volume is still attached to a node because
	// kubelet is slowly cleaning up the previous pod, however it should succeed
	// in a couple of minutes.
	if t.persistentVolume != nil && t.persistentVolume.Spec.PersistentVolumeReclaimPolicy == v1.PersistentVolumeReclaimDelete {
		framework.Logf("deleting PVC [%s/%s] using PV [%s]", t.namespace.Name, t.persistentVolumeClaim.Name, t.persistentVolume.Name)
		v2.By(fmt.Sprintf("waiting for claim's PV [%s] to be deleted", t.persistentVolume.Name))
		err := k8sDevPV.WaitForPersistentVolumeDeleted(t.client, t.persistentVolume.Name, 5*time.Second, 10*time.Minute)
		framework.ExpectNoError(err)
	}
	// Wait for the PVC to be deleted
	//err = framework.WaitForPersistentVolumeClaimDeleted(t.client, t.persistentVolumeClaim.Name, t.namespace.Name, 5*time.Second, 5*time.Minute)
	framework.ExpectNoError(err)
}

func (t *TestPersistentVolumeClaim) WaitForBound() v1.PersistentVolumeClaim {
	var err error

	v2.By(fmt.Sprintf("waiting for PVC to be in phase %q", v1.ClaimBound))
	err = k8sDevPV.WaitForPersistentVolumeClaimPhase(v1.ClaimBound, t.client, t.namespace.Name, t.persistentVolumeClaim.Name, framework.Poll, framework.ClaimProvisionTimeout)
	framework.ExpectNoError(err)

	v2.By("checking the PVC")
	// Get new copy of the claim
	t.persistentVolumeClaim, err = t.client.CoreV1().PersistentVolumeClaims(t.namespace.Name).Get(context.Background(), t.persistentVolumeClaim.Name, metav1.GetOptions{})
	framework.ExpectNoError(err)

	return *t.persistentVolumeClaim
}

func (t *TestPersistentVolumeClaim) ValidateProvisionedPersistentVolume() {
	var err error

	// Get the bound PersistentVolume
	v2.By("validating provisioned PV")
	t.persistentVolume, err = t.client.CoreV1().PersistentVolumes().Get(context.Background(), t.persistentVolumeClaim.Spec.VolumeName, metav1.GetOptions{})
	framework.ExpectNoError(err)
	framework.Logf("validating provisioned PV [%s] for PVC [%s]", t.persistentVolume.Name, t.persistentVolumeClaim.Name)

	// Check sizes
	expectedCapacity := t.requestedPersistentVolumeClaim.Spec.Resources.Requests[v1.ResourceName(v1.ResourceStorage)]
	claimCapacity := t.persistentVolumeClaim.Spec.Resources.Requests[v1.ResourceName(v1.ResourceStorage)]
	gomega.Expect(claimCapacity.Value()).To(gomega.Equal(expectedCapacity.Value()), "claimCapacity is not equal to requestedCapacity")

	pvCapacity := t.persistentVolume.Spec.Capacity[v1.ResourceStorage]
	gomega.Expect(pvCapacity.Value()).To(gomega.Equal(expectedCapacity.Value()), "pvCapacity is not equal to requestedCapacity")

	// Check PV properties
	v2.By("checking PV")
	framework.Logf("checking PV [%s]", t.persistentVolume.Name)
	expectedAccessModes := t.requestedPersistentVolumeClaim.Spec.AccessModes
	gomega.Expect(t.persistentVolume.Spec.AccessModes).To(gomega.Equal(expectedAccessModes))
	gomega.Expect(t.persistentVolume.Spec.ClaimRef.Name).To(gomega.Equal(t.persistentVolumeClaim.ObjectMeta.Name))
	gomega.Expect(t.persistentVolume.Spec.ClaimRef.Namespace).To(gomega.Equal(t.persistentVolumeClaim.ObjectMeta.Namespace))
}

func generatePVC(name, namespace,
	storageClassName string, claimSize string,
	accessMode v1.PersistentVolumeAccessMode) *v1.PersistentVolumeClaim {
	var objMeta metav1.ObjectMeta
	lastChar := name[len(name)-1:]
	if lastChar == "-" {
		objMeta = metav1.ObjectMeta{
			GenerateName: name,
			Namespace:    namespace,
			Labels:       map[string]string{"app": "obj-e2e-tester"},
		}
	} else {
		objMeta = metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    map[string]string{"app": "obj-e2e-tester"},
		}
	}
	return &v1.PersistentVolumeClaim{
		ObjectMeta: objMeta,
		Spec: v1.PersistentVolumeClaimSpec{
			StorageClassName: &storageClassName,
			AccessModes: []v1.PersistentVolumeAccessMode{
				accessMode,
			},
			Resources: v1.ResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceName(v1.ResourceStorage): resource.MustParse(claimSize),
				},
			},
		},
	}
}

func (t *TestPod) Cleanup() {
	v2.By("POD Cleanup: deleting POD")
	framework.Logf("deleting POD [%s/%s]", t.namespace.Name, t.pod.Name)
	cleanupPodOrFail(t.client, t.pod.Name, t.namespace.Name, t.dumpDbgInfo, t.dumpLog)
}

func cleanupPodOrFail(client clientset.Interface, name, namespace string, dbginfo, log bool) {
	if dbginfo {
		e2eoutput.DumpDebugInfo(client, namespace)
	}
	if log {
		body, err := podLogs(client, name, namespace)
		if err != nil {
			framework.Logf("Error getting logs for pod %s: %v", name, err)
		} else {
			framework.Logf("Pod %s has the following logs: %s", name, body)
		}
	}
	framework.Logf("deleting POD [%s/%s]", namespace, name)
	k8sDevPod.DeletePodOrFail(client, namespace, name)
}

func podLogs(client clientset.Interface, name, namespace string) ([]byte, error) {
	return client.CoreV1().Pods(namespace).GetLogs(name, &v1.PodLogOptions{}).Do(context.Background()).Raw()
}
