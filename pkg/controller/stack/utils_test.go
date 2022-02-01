package stack

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_WithInferNamespace(t *testing.T) {

	os.Setenv(INFERNS, "1")
	defer os.Unsetenv(INFERNS)

	assert.Equal(t, "namespace: test-ns", inferNamespace("test-ns"))

}

func Test_WithoutInferNamespace(t *testing.T) {
	assert.Equal(t, "", inferNamespace("test-ns"))
}
