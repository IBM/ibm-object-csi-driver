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
	"errors"
	"fmt"
	v2 "github.com/onsi/ginkgo/v2"
	v1 "k8s.io/api/core/v1"
	clientset "k8s.io/client-go/kubernetes"
)

// DynamicallyProvisioneVolPodWRTest will provision required PVC and Deployment
// Testing if the Pod can write and read to mounted volumes
type DynamicallyProvisionePodWithVolTest struct {
	Pods     []PodDetails
	PodCheck *PodExecCheck
}

func (t *DynamicallyProvisionePodWithVolTest) Run(client clientset.Interface, namespace *v1.Namespace) error {
	var testFailed bool
	for n, pod := range t.Pods {
		tpod, cleanup := pod.SetupWithDynamicVolumes(client, namespace)
		// defer must be called here for resources not get removed before using them
		for i := range cleanup {
			defer cleanup[i]()
		}
		ind := fmt.Sprintf("%d", n)
		v2.By(fmt.Sprintf("deploying the pod %q", ind))
		tpod.Create()
		defer tpod.Cleanup()

		if !pod.CmdExits {
			v2.By("checking that the pods status is running")
			if err := tpod.WaitForRunningSlow(); err != nil {
				// If WaitForRunningSlow fails, set testFailed to true
				testFailed = true
				break
			}
			if t.PodCheck != nil {
				v2.By("checking pod exec after pod recreate")
				if err := tpod.Exec(t.PodCheck.Cmd, t.PodCheck.ExpectedString01); err != nil {
					v2.By("Pod Exec failed")
					testFailed = true
					break
				}
			}
		} else {
			v2.By("checking that the pods command exits with no error")
			if err := tpod.WaitForSuccess(); err != nil {

				testFailed = true
				break
			}
		}
	}
	// If testFailed is true, return an error
	if testFailed {
		v2.By("Return Error")
		return errors.New("test failed")
	}
	// Otherwise, return nil
	return nil
}
