// Copyright 2021, Pulumi Corporation.  All rights reserved.
package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_WithInferNamespace(t *testing.T) {
	t.Setenv(INFERNS, "1")

	assert.Equal(t, "namespace: test-ns", inferNamespace("test-ns"))
}

func Test_WithoutInferNamespace(t *testing.T) {
	assert.Equal(t, "", inferNamespace("test-ns"))
}
