/*******************************************************************************
 * IBM Confidential
 * OCO Source Materials
 * IBM Cloud Kubernetes Service, 5737-D43
 * (C) Copyright IBM Corp. 2024 All Rights Reserved.
 * The source code for this program is not published or otherwise divested of
 * its trade secrets, irrespective of what has been deposited with
 * the U.S. Copyright Office.
 ******************************************************************************/

package utils

import "fmt"

var exists = struct{}{}

// Set is a collection of unique string items
type Set struct {
	m map[string]struct{}
}

// NewSet creates an empty Set
func NewSet() *Set {
	return &Set{
		m: make(map[string]struct{}),
	}
}

// NewSetWithValues creates a Set with initial values
func NewSetWithValues(values ...string) *Set {
	s := &Set{
		m: make(map[string]struct{}, len(values)),
	}
	for _, v := range values {
		s.m[v] = exists
	}
	return s
}

// Add adds an item to the Set
func (s *Set) Add(key string) {
	s.m[key] = exists
}

// Remove removes an item from the Set
func (s *Set) Remove(key string) error {
	if _, ok := s.m[key]; !ok {
		return fmt.Errorf("remove error: item '%s' doesn't exist in the set", key)
	}
	delete(s.m, key)
	return nil
}

// Contains checks if an item exists in the Set
func (s *Set) Contains(key string) bool {
	_, ok := s.m[key]
	return ok
}

// Size returns the number of items in the Set
func (s *Set) Size() int {
	return len(s.m)
}

