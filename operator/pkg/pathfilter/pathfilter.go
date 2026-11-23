// Copyright 2026, Pulumi Corporation.
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

// Package pathfilter matches changed file paths against a set of include and exclude globs.
package pathfilter

import (
	"fmt"
	"strings"

	"github.com/gobwas/glob"
)

// Filter is a single path filter glob and whether it's a negated (exclude) filter.
type Filter struct {
	exclude bool // true if the filter string has a leading "!"
	glob    glob.Glob
}

// Compile compiles path filter strings and reports if all filter strings are exclude filters.
func Compile(filterStrings []string) (filters []Filter, allExcludes bool, err error) {
	filters = make([]Filter, len(filterStrings))
	allExcludes = true
	for i, f := range filterStrings {
		exclude := strings.HasPrefix(f, "!")
		if exclude {
			f = f[1:]
		}
		allExcludes = allExcludes && exclude

		g, err := glob.Compile(f)
		if err != nil {
			return nil, false, fmt.Errorf("compiling filter: %w", err)
		}

		filters[i] = Filter{exclude: exclude, glob: g}
	}
	return filters, allExcludes, nil
}

// Matches reports whether a path is included in the filters. A path is included if at least one include
// filter matches it and no exclude filter does.
//
// Special case: If there are only exclude filters in the filters list, implicitly all other paths are included.
func Matches(filters []Filter, allExcludes bool, path string) bool {
	included := allExcludes
	for _, filter := range filters {
		if !filter.glob.Match(path) {
			continue
		}
		if filter.exclude {
			return false
		}
		included = true
	}
	return included
}
