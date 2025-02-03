package main

import (
	"testing"

	"github.com/dell/csm-operator/pkg/logger"
)

func TestPrintVersion(t *testing.T) {
	_, log := logger.GetNewContextWithLogger("main")

	printVersion(log)
}

func TestMain(t *testing.T) {
	//main()
}
