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
	"strings"
)

// maskSensitive masks sensitive strings for logging
func maskSensitive(s string) string {
	if len(s) <= 4 {
		return "****"
	}
	return s[:2] + strings.Repeat("*", len(s)-4) + s[len(s)-2:]
}

// maskArgs masks sensitive arguments in a slice for logging
func maskArgs(args []string) []string {
	masked := make([]string, len(args))
	for i, arg := range args {
		// Mask values that look like credentials, keys, tokens, etc.
		if strings.Contains(strings.ToLower(arg), "key") ||
			strings.Contains(strings.ToLower(arg), "secret") ||
			strings.Contains(strings.ToLower(arg), "token") ||
			strings.Contains(strings.ToLower(arg), "password") ||
			strings.Contains(strings.ToLower(arg), "credential") {
			masked[i] = maskSensitive(arg)
		} else {
			masked[i] = arg
		}
	}
	return masked
}
