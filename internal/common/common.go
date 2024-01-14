package common

import (
	"fmt"
	danav1 "github.com/dana-team/hns/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DeletionTimeStampExists returns true if an object is being deleted, and false otherwise.
func DeletionTimeStampExists(object client.Object) bool {
	return !object.GetDeletionTimestamp().IsZero()
}

// IndexOf returns the index of the given element in the given array of strings.
func IndexOf(element string, a []string) (int, error) {
	for key, value := range a {
		if element == value {
			return key, nil
		}
	}

	return -1, fmt.Errorf("failed to find element %q in slice", element)
}

// ContainsString checks if a string is present in a slice of strings.
func ContainsString(sslice []string, s string) bool {
	for _, a := range sslice {
		if a == s {
			return true
		}
	}

	return false
}

// ShouldReconcile returns true if the Phase given as argument is
// not Complete or Error; meaning that reconciliation needs to take place.
func ShouldReconcile(phase danav1.Phase) bool {
	return phase != danav1.Complete && phase != danav1.Error
}
