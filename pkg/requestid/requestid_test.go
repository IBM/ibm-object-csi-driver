/*******************************************************************************
 * IBM Confidential
 * OCO Source Materials
 * IBM Cloud Kubernetes Service, 5737-D43
 * (C) Copyright IBM Corp. 2023 All Rights Reserved.
 * The source code for this program is not published or otherwise divested of
 * its trade secrets, irrespective of what has been deposited with
 * the U.S. Copyright Office.
 ******************************************************************************/

package requestid

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestGenerate(t *testing.T) {
	requestID := Generate()
	assert.NotEmpty(t, requestID)
	
	// Verify it's a valid UUID
	_, err := uuid.Parse(requestID)
	assert.NoError(t, err)
	
	// Verify uniqueness
	requestID2 := Generate()
	assert.NotEqual(t, requestID, requestID2)
}

func TestWithRequestID(t *testing.T) {
	ctx := context.Background()
	testID := "test-request-id-123"
	
	ctx = WithRequestID(ctx, testID)
	retrievedID := FromContext(ctx)
	
	assert.Equal(t, testID, retrievedID)
}

func TestWithRequestIDEmpty(t *testing.T) {
	ctx := context.Background()
	
	// Empty string should generate a new ID
	ctx = WithRequestID(ctx, "")
	retrievedID := FromContext(ctx)
	
	assert.NotEmpty(t, retrievedID)
	_, err := uuid.Parse(retrievedID)
	assert.NoError(t, err)
}

func TestFromContext(t *testing.T) {
	t.Run("with request ID", func(t *testing.T) {
		ctx := context.Background()
		testID := "test-id-456"
		ctx = WithRequestID(ctx, testID)
		
		retrievedID := FromContext(ctx)
		assert.Equal(t, testID, retrievedID)
	})
	
	t.Run("without request ID", func(t *testing.T) {
		ctx := context.Background()
		retrievedID := FromContext(ctx)
		assert.Empty(t, retrievedID)
	})
	
	t.Run("nil context", func(t *testing.T) {
		retrievedID := FromContext(nil)
		assert.Empty(t, retrievedID)
	})
}

func TestGetOrGenerate(t *testing.T) {
	t.Run("existing request ID", func(t *testing.T) {
		ctx := context.Background()
		existingID := "existing-id-789"
		ctx = WithRequestID(ctx, existingID)
		
		newCtx, requestID := GetOrGenerate(ctx)
		assert.Equal(t, existingID, requestID)
		assert.Equal(t, existingID, FromContext(newCtx))
	})
	
	t.Run("no existing request ID", func(t *testing.T) {
		ctx := context.Background()
		
		newCtx, requestID := GetOrGenerate(ctx)
		assert.NotEmpty(t, requestID)
		assert.Equal(t, requestID, FromContext(newCtx))
		
		// Verify it's a valid UUID
		_, err := uuid.Parse(requestID)
		assert.NoError(t, err)
	})
	
	t.Run("nil context", func(t *testing.T) {
		newCtx, requestID := GetOrGenerate(nil)
		assert.NotNil(t, newCtx)
		assert.NotEmpty(t, requestID)
		assert.Equal(t, requestID, FromContext(newCtx))
	})
}

func TestMustGet(t *testing.T) {
	t.Run("with request ID", func(t *testing.T) {
		ctx := context.Background()
		testID := "must-get-test-id"
		ctx = WithRequestID(ctx, testID)
		
		retrievedID := MustGet(ctx)
		assert.Equal(t, testID, retrievedID)
	})
	
	t.Run("without request ID panics", func(t *testing.T) {
		ctx := context.Background()
		
		assert.Panics(t, func() {
			MustGet(ctx)
		})
	})
}


