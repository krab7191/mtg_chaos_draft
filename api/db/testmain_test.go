package db_test

import (
	"os"
	"testing"

	"mtg-chaos-draft/testhelper"
)

func TestMain(m *testing.M) {
	testhelper.Setup()
	os.Exit(m.Run())
}
