/*******************************************************************************
 * IBM Confidential
 * OCO Source Materials
 * IBM Cloud Kubernetes Service, 5737-D43
 * (C) Copyright IBM Corp. 2023 All Rights Reserved.
 * The source code for this program is not published or otherwise divested of
 * its trade secrets, irrespective of what has been deposited with
 * the U.S. Copyright Office.
 ******************************************************************************/

// Package mounter
package mounter

import (
	"errors"
	"os/exec"
	"strings"
	"time"

	"k8s.io/klog/v2"
)

func waitForMount(path string, timeout time.Duration) error {
	var elapsed time.Duration
	var interval = 10 * time.Millisecond
	for {
		out, err := exec.Command("mountpoint", path).CombinedOutput()
		outStr := strings.TrimSpace(string(out))
		if err != nil {
			return err
		}
		if strings.HasSuffix(outStr, "is a mountpoint") {
			klog.Infof("Path is a mountpoint: pathname - %s", path)
			return nil
		}

		time.Sleep(interval)
		elapsed = elapsed + interval
		if elapsed >= timeout {
			return errors.New("timeout waiting for mount")
		}
	}
}
