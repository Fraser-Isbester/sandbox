package chat

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewHandler(t *testing.T) {
	h := NewHandler()
	assert.NotNil(t, h)
}
