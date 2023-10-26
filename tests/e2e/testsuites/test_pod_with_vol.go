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
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	v1 "k8s.io/api/core/v1"
	clientset "k8s.io/client-go/kubernetes"
)

// DynamicallyProvisioneVolPodWRTest will provision required PVC and Deployment
// Testing if the Pod can write and read to mounted volumes
type DynamicallyProvisionePodWithVolTest struct {
	Pods     []PodDetails
	PodCheck *PodExecCheck
}

func (t *DynamicallyProvisionePodWithVolTest) Run(client clientset.Interface, namespace *v1.Namespace) {
	for n, pod := range t.Pods {
		tpod, cleanup := pod.SetupWithDynamicVolumes(client, namespace)
		// defer must be called here for resources not get removed before using them
		for i := range cleanup {
			defer cleanup[i]()
		}
		ind := fmt.Sprintf("%d", n)
		By(fmt.Sprintf("deploying the pod %q", ind))
		tpod.Create()
		defer tpod.Cleanup()

		if !pod.CmdExits {
			By("checking that the pods status is running")
			tpod.WaitForRunningSlow()
			if t.PodCheck != nil {
				By("checking pod exec after pod recreate")
				tpod.Exec(t.PodCheck.Cmd, t.PodCheck.ExpectedString01)
			}
		} else {
			By("checking that the pods command exits with no error")
			tpod.WaitForSuccess()
		}
	}
}
