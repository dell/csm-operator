package main

import (
	"testing"

	"github.com/dell/csm-operator/pkg/logger"
	"github.com/stretchr/testify/assert"
)

func TestPrintVersion(t *testing.T) {
	_, log := logger.GetNewContextWithLogger("main")
	// TODO: Hook onto the output and verify that it matches the expected
	printVersion(log)
}

func TestGetOperatorConfig(t *testing.T) {
	_, log := logger.GetNewContextWithLogger("main")

	testConfig := getOperatorConfig(log)
	assert.NotNil(t, testConfig)

	// TODO: error test cases

}
