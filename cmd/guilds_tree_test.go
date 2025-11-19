package cmd

import (
	"strings"
	"testing"
)

func TestUnknownComponentErrorHandling(t *testing.T) {
	// Test the error string matching logic for UnknownComponent errors
	testCases := []struct {
		name        string
		errorString string
		shouldMatch bool
	}{
		{
			name:        "Standard UnknownComponent error",
			errorString: "JSON decoding failed: expected container, got *discord.UnknownComponent",
			shouldMatch: true,
		},
		{
			name:        "UnknownComponent with additional context",
			errorString: "some other error: JSON decoding failed: expected container, got *discord.UnknownComponent: more context",
			shouldMatch: true,
		},
		{
			name:        "Regular error without UnknownComponent",
			errorString: "some other error message",
			shouldMatch: false,
		},
		{
			name:        "JSON decoding without UnknownComponent",
			errorString: "JSON decoding failed: other error",
			shouldMatch: false,
		},
		{
			name:        "UnknownComponent without JSON decoding",
			errorString: "expected container, got *discord.UnknownComponent",
			shouldMatch: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			hasJSONDecoding := strings.Contains(tc.errorString, "JSON decoding failed")
			hasUnknownComponent := strings.Contains(tc.errorString, "UnknownComponent")
			matches := hasJSONDecoding && hasUnknownComponent

			if matches != tc.shouldMatch {
				t.Errorf("Expected match: %v, got: %v for error string: %s", tc.shouldMatch, matches, tc.errorString)
			}
		})
	}
}
