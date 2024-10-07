// Copyright 2016-2024, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package pulumi

import "testing"

func TestExactlyOneOf(t *testing.T) {
	tests := []struct {
		name     string
		input    []bool
		expected bool
	}{
		{
			name:     "No true values",
			input:    []bool{false, false, false},
			expected: false,
		},
		{
			name:     "One true value",
			input:    []bool{false, true, false},
			expected: true,
		},
		{
			name:     "Multiple true values",
			input:    []bool{true, true, false},
			expected: false,
		},
		{
			name:     "All true values",
			input:    []bool{true, true, true},
			expected: false,
		},
		{
			name:     "Empty input",
			input:    []bool{},
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := exactlyOneOf(tc.input...)
			if result != tc.expected {
				t.Errorf("exactlyOneOf(%v) = %v; want %v", tc.input, result, tc.expected)
			}
		})
	}
}
