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
package e2e

import (
	"context"
	"fmt"
	"os"

	"github.com/IBM/ibm-object-csi-driver/tests/e2e/testsuites"
	. "github.com/onsi/ginkgo/v2"
	gtypes "github.com/onsi/ginkgo/v2/types"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clientset "k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	"k8s.io/kubernetes/test/e2e/framework"
	admissionapi "k8s.io/pod-security-admission/api"
)

// ENV required for testsuite execution
var cosEndpoint = os.Getenv("cosEndpoint")
var locationConstraint = os.Getenv("locationConstraint")
var s3fsBucketName = os.Getenv("s3fsBucketName")
var accessKey = os.Getenv("accessKey")
var secretKey = os.Getenv("secretKey")
var rcloneBucketName = os.Getenv("rcloneBucketName")

const (
	driverName   = "cos-s3-csi-driver"
	s3fsscName   = "ibm-object-storage-smart-s3fs"
	rclonescName = "ibm-object-storage-smart-rclone"
)

var _ = Describe("rclone", func() {
	f := framework.NewDefaultFramework("obj-e2e-rclone")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged
	var (
		cs clientset.Interface
		ns *v1.Namespace
	)

	BeforeEach(func() {
		cs = f.ClientSet
		ns = f.Namespace
	})

	It("with rclone SC, Create a volume - Attach to a Pod  - Read/Write", func() {
		payload := `{"metadata": {"labels": {"security.openshift.io/scc.podSecurityLabelSync": "false","pod-security.kubernetes.io/enforce": "privileged"}}}`
		_, labelerr := cs.CoreV1().Namespaces().Patch(context.TODO(), ns.Name, types.StrategicMergePatchType, []byte(payload), metav1.PatchOptions{})
		if labelerr != nil {
			panic(labelerr)
		}
		secret := testsuites.NewSecret(cs, ns.Name, driverName, ns.Name, cosEndpoint, locationConstraint, rcloneBucketName, accessKey, secretKey)
		secret.Create()
		defer secret.Cleanup()

		pod := []testsuites.PodDetails{
			{
				Cmd:      "echo 'hello world' >> /mnt/test-1/data && while true; do sleep 2; done",
				CmdExits: false,
				Volumes: []testsuites.VolumeDetails{
					{
						PVCName:    ns.Name,
						VolumeType: rclonescName,
						ClaimSize:  "256Mi",
						VolumeMount: testsuites.VolumeMountDetails{
							NameGenerate:      "test-volume-",
							MountPathGenerate: "/mnt/test-",
						},
					},
				},
			},
		}
		test := testsuites.DynamicallyProvisionePodWithVolTest{
			Pods: pod,
			PodCheck: &testsuites.PodExecCheck{
				Cmd:              []string{"cat", "/mnt/test-1/data"},
				ExpectedString01: "hello world\n",
				ExpectedString02: "hello world\nhello world\n", // pod will be restarted so expect to see 2 instances of string
			},
		}
		customReporter, err := NewCustomReporter("test_results.txt")
		if err != nil {
			println("Failed to create file:", err)
			return
		}
		defer customReporter.Close()

		err = test.Run(cs, ns)
		if err != nil {
			// If Run returns an error, report the failure
			fmt.Println("Test failed:", err)
			customReporter.SpecDidComplete(&gtypes.SpecSummary{
				State:          gtypes.SpecStateFailed,
				ComponentTexts: []string{"DynamicallyProvisionePodWithVolTest RCLONE"},
			})
		} else {
			// If Run succeeds, report the success
			fmt.Println("Test passed")
			customReporter.SpecDidComplete(&gtypes.SpecSummary{
				State:          gtypes.SpecStatePassed,
				ComponentTexts: []string{"DynamicallyProvisionePodWithVolTest RCLONE"},
			})
		}
	})

})

var _ = Describe("s3fs", func() {
	f := framework.NewDefaultFramework("obj-e2e-s3fs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged
	var (
		cs clientset.Interface
		ns *v1.Namespace
	)

	BeforeEach(func() {
		cs = f.ClientSet
		ns = f.Namespace
	})

	It("with s3fs SC, Create a volume - Attach to a Pod  - Read/Write", func() {
		payload := `{"metadata": {"labels": {"security.openshift.io/scc.podSecurityLabelSync": "false","pod-security.kubernetes.io/enforce": "privileged"}}}`
		_, labelerr := cs.CoreV1().Namespaces().Patch(context.TODO(), ns.Name, types.StrategicMergePatchType, []byte(payload), metav1.PatchOptions{})
		if labelerr != nil {
			panic(labelerr)
		}
		secret := testsuites.NewSecret(cs, ns.Name, driverName, ns.Name, cosEndpoint, locationConstraint, s3fsBucketName, accessKey, secretKey)
		secret.Create()
		defer secret.Cleanup()

		pod := []testsuites.PodDetails{
			{
				Cmd:      "echo 'hello world' >> /mnt/test-1/data && while true; do sleep 2; done",
				CmdExits: false,
				Volumes: []testsuites.VolumeDetails{
					{
						PVCName:    ns.Name,
						VolumeType: s3fsscName,
						ClaimSize:  "256Mi",
						VolumeMount: testsuites.VolumeMountDetails{
							NameGenerate:      "test-volume-",
							MountPathGenerate: "/mnt/test-",
						},
					},
				},
			},
		}
		test := testsuites.DynamicallyProvisionePodWithVolTest{
			Pods: pod,
			PodCheck: &testsuites.PodExecCheck{
				Cmd:              []string{"cat", "/mnt/test-1/data"},
				ExpectedString01: "hello world\n",
				ExpectedString02: "hello world\nhello world\n", // pod will be restarted so expect to see 2 instances of string
			},
		}
		customReporter, err := NewCustomReporter("test_results.txt")
		if err != nil {
			println("Failed to create file:", err)
			return
		}
		defer customReporter.Close()

		err = test.Run(cs, ns)
		if err != nil {
			// If Run returns an error, report the failure
			fmt.Println("Test failed:", err)
			customReporter.SpecDidComplete(&gtypes.SpecSummary{
				State:          gtypes.SpecStateFailed,
				ComponentTexts: []string{"DynamicallyProvisionePodWithVolTest S3FS"},
			})
		} else {
			// If Run succeeds, report the success
			fmt.Println("Test passed")
			customReporter.SpecDidComplete(&gtypes.SpecSummary{
				State:          gtypes.SpecStatePassed,
				ComponentTexts: []string{"DynamicallyProvisionePodWithVolTest S3FS"},
			})
		}
	})

})
