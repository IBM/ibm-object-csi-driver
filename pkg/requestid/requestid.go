/*******************************************************************************
 * IBM Confidential
 * OCO Source Materials
 * IBM Cloud Kubernetes Service, 5737-D43
 * (C) Copyright IBM Corp. 2023 All Rights Reserved.
 * The source code for this program is not published or otherwise divested of
 * its trade secrets, irrespective of what has been deposited with
 * the U.S. Copyright Office.
 ******************************************************************************/

// Package requestid provides utilities for generating and managing request IDs
// throughout the CSI driver lifecycle for better debugging and log correlation.
package requestid

import (
	"context"

	"github.com/google/uuid"
)

type contextKey string

const (
	// RequestIDKey is the context key for storing request IDs
	RequestIDKey contextKey = "request-id"
)

// Generate creates a new UUID v4 request ID
func Generate() string {
	return uuid.New().String()
}

// WithRequestID adds a request ID to the context
func WithRequestID(ctx context.Context, requestID string) context.Context {
	if requestID == "" {
		requestID = Generate()
	}
	return context.WithValue(ctx, RequestIDKey, requestID)
}

// FromContext extracts the request ID from context
// Returns empty string if no request ID is found
func FromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if requestID, ok := ctx.Value(RequestIDKey).(string); ok {
		return requestID
	}
	return ""
}

// GetOrGenerate gets the request ID from context or generates a new one
// Returns the updated context and the request ID
func GetOrGenerate(ctx context.Context) (context.Context, string) {
	if ctx == nil {
		ctx = context.Background()
	}
	
	requestID := FromContext(ctx)
	if requestID == "" {
		requestID = Generate()
		ctx = WithRequestID(ctx, requestID)
	}
	return ctx, requestID
}

// MustGet extracts request ID from context, panics if not found
// This should only be used in scenarios where request ID is guaranteed to exist
func MustGet(ctx context.Context) string {
	requestID := FromContext(ctx)
	if requestID == "" {
		panic("request ID not found in context")
	}
	return requestID
}


