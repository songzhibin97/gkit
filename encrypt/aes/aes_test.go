package aes

import "testing"

func TestPadKeyToLength(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		targetLength int
		expected     string
	}{
		{
			name:         "Empty input with defaultKey",
			input:        "",
			targetLength: 8,
			expected:     "gkitgkit", // Assuming defaultKey is defined globally
		},
		{
			name:         "Input shorter than target length",
			input:        "abc",
			targetLength: 8,
			expected:     "abcabcab",
		},
		{
			name:         "Input equal to target length",
			input:        "abcdefgh",
			targetLength: 8,
			expected:     "abcdefgh",
		},
		{
			name:         "Input longer than target length",
			input:        "abcdefghijklmnopqrstuvwxyz",
			targetLength: 8,
			expected:     "abcdefgh",
		},
		{
			name:         "Empty input with zero target length",
			input:        "",
			targetLength: 0,
			expected:     "",
		},
		{
			name:         "Non-empty input with zero target length",
			input:        "abc",
			targetLength: 0,
			expected:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PadKeyToLength(tt.input, tt.targetLength)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}
