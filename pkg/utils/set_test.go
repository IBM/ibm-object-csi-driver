/*******************************************************************************
 * IBM Confidential
 * OCO Source Materials
 * IBM Cloud Kubernetes Service, 5737-D43
 * (C) Copyright IBM Corp. 2026 All Rights Reserved.
 * The source code for this program is not published or otherwise divested of
 * its trade secrets, irrespective of what has been deposited with
 * the U.S. Copyright Office.
 ******************************************************************************/

package utils

import (
	"testing"
)

func TestNewSet(t *testing.T) {
	s := NewSet()
	if s == nil {
		t.Error("NewSet() returned nil")
	}
	if s.Size() != 0 {
		t.Errorf("NewSet() size = %d, want 0", s.Size())
	}
}

func TestNewSetWithValues(t *testing.T) {
	tests := []struct {
		name   string
		values []string
		want   int
	}{
		{"Empty", []string{}, 0},
		{"Single", []string{"a"}, 1},
		{"Multiple", []string{"a", "b", "c"}, 3},
		{"Duplicates", []string{"a", "b", "a", "c", "b"}, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewSetWithValues(tt.values...)
			if got := s.Size(); got != tt.want {
				t.Errorf("NewSetWithValues() size = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestSet_Add(t *testing.T) {
	s := NewSet()

	// Add first item
	s.Add("a")
	if !s.Contains("a") {
		t.Error("Add() failed to add item 'a'")
	}
	if s.Size() != 1 {
		t.Errorf("Add() size = %d, want 1", s.Size())
	}

	// Add duplicate
	s.Add("a")
	if s.Size() != 1 {
		t.Errorf("Add() duplicate increased size to %d, want 1", s.Size())
	}

	// Add second item
	s.Add("b")
	if !s.Contains("b") {
		t.Error("Add() failed to add item 'b'")
	}
	if s.Size() != 2 {
		t.Errorf("Add() size = %d, want 2", s.Size())
	}
}

func TestSet_Contains(t *testing.T) {
	s := NewSetWithValues("a", "b", "c")

	tests := []struct {
		name string
		key  string
		want bool
	}{
		{"Exists_a", "a", true},
		{"Exists_b", "b", true},
		{"Exists_c", "c", true},
		{"NotExists_d", "d", false},
		{"NotExists_empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := s.Contains(tt.key); got != tt.want {
				t.Errorf("Contains(%q) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}

func TestSet_Remove(t *testing.T) {
	tests := []struct {
		name     string
		initial  []string
		remove   string
		wantErr  bool
		wantSize int
	}{
		{"RemoveExisting", []string{"a", "b", "c"}, "b", false, 2},
		{"RemoveNonExisting", []string{"a", "b"}, "c", true, 2},
		{"RemoveFromEmpty", []string{}, "a", true, 0},
		{"RemoveLast", []string{"a"}, "a", false, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewSetWithValues(tt.initial...)
			err := s.Remove(tt.remove)

			if (err != nil) != tt.wantErr {
				t.Errorf("Remove() error = %v, wantErr %v", err, tt.wantErr)
			}

			if got := s.Size(); got != tt.wantSize {
				t.Errorf("Remove() size = %d, want %d", got, tt.wantSize)
			}

			if !tt.wantErr && s.Contains(tt.remove) {
				t.Errorf("Remove() item %q still exists in set", tt.remove)
			}
		})
	}
}

func TestSet_Size(t *testing.T) {
	tests := []struct {
		name   string
		values []string
		want   int
	}{
		{"Empty", []string{}, 0},
		{"One", []string{"a"}, 1},
		{"Multiple", []string{"a", "b", "c", "d", "e"}, 5},
		{"WithDuplicates", []string{"a", "b", "a", "c", "b", "d"}, 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewSetWithValues(tt.values...)
			if got := s.Size(); got != tt.want {
				t.Errorf("Size() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestSet_Operations(t *testing.T) {
	// Test complex operations
	s := NewSet()

	// Add multiple items
	items := []string{"allow_other", "auto_cache", "cipher_suites", "connect_timeout"}
	for _, item := range items {
		s.Add(item)
	}

	if s.Size() != 4 {
		t.Errorf("After adding 4 items, size = %d, want 4", s.Size())
	}

	// Check all items exist
	for _, item := range items {
		if !s.Contains(item) {
			t.Errorf("Contains(%q) = false, want true", item)
		}
	}

	// Remove one item
	if err := s.Remove("auto_cache"); err != nil {
		t.Errorf("Remove() error = %v, want nil", err)
	}

	if s.Size() != 3 {
		t.Errorf("After removing 1 item, size = %d, want 3", s.Size())
	}

	if s.Contains("auto_cache") {
		t.Error("Contains(auto_cache) = true after removal, want false")
	}

	// Add duplicate
	s.Add("allow_other")
	if s.Size() != 3 {
		t.Errorf("After adding duplicate, size = %d, want 3", s.Size())
	}
}
