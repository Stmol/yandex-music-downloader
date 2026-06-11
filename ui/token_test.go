package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTokenResizeExpandsInputToWindowWidth(t *testing.T) {
	m := NewTokenModel(nil)

	m.Resize(120, 40)

	assert.Equal(t, 116, m.inputField.Width())
}

func TestTokenResizeKeepsMinimumInputWidth(t *testing.T) {
	m := NewTokenModel(nil)

	m.Resize(10, 40)

	assert.Equal(t, minInputWidth, m.inputField.Width())
}
