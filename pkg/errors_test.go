// Copyright (c) Bas van Beek 2022.
// Copyright (c) Tetrate, Inc 2021.
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

// pkg_test provides unit tests for the `pkg` package. It is intentionally renamed
// to avoid pulling unnecessary dependencies into the project.
package pkg_test

import (
	"errors"
	"fmt"
	"testing"

	hme "github.com/hashicorp/go-multierror"
	ghe "github.com/pkg/errors"
	"github.com/tetratelabs/multierror"

	"github.com/basvanbeek/topology-tester/pkg"
)

var (
	errSimple  = errors.New("simple")
	errWrapped = fmt.Errorf("wrapped error: %w", errSimple)
	errOldWrap = ghe.Wrap(errWrapped, "github.com/pkg/errors wrapped")

	multi        error
	errMultiHME  error
	errWrapMulti error
)

func TestHasError(t *testing.T) {
	multi = multierror.Append(multi, errors.New("other"), errOldWrap)
	errMultiHME = hme.Append(errMultiHME, errors.New("other"), errOldWrap)
	errWrapMulti = fmt.Errorf("level 1 %w", fmt.Errorf("level 2 %w", multi))

	tests := []struct {
		name     string
		in       error
		target   error
		expected bool
	}{
		{"nil", nil, errSimple, false},
		{"nil-expected", nil, nil, true},
		{"nil-unexpected", errSimple, nil, false},
		{"simple", errSimple, errSimple, true},
		{"wrapped", errWrapped, errSimple, true},
		{"old-wrapped", errOldWrap, errSimple, true},
		{"no-deep", errOldWrap, errWrapped, true},
		{"multi", multi, errSimple, true},
		{"hme-multi", errMultiHME, errSimple, true},
		{"wrap-multi", errWrapMulti, errSimple, true},
		{"no-has", errWrapped, errOldWrap, false},
		{"no-equals", errSimple, errors.New("simple"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			has := pkg.HasError(tt.in, tt.target)
			if has != tt.expected {
				t.Errorf("expected %t, got %t", tt.expected, has)
			}
		})
	}
}
