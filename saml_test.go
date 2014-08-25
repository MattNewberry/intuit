package intuit

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestSaml(t *testing.T) {
	token, err := MakeSamlAssertion()
	assert.NoError(t, err)
	assert.NotEmpty(t, token)
}
