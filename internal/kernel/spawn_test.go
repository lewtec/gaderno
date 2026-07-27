package kernel

import (
	"errors"
	"testing"
)

func TestStartProcessEmptyArgv(t *testing.T) {
	_, err := StartProcess(Spec{}, "", t.TempDir())
	if !errors.Is(err, ErrEmptyArgv) {
		t.Fatalf("StartProcess empty argv: got %v, want ErrEmptyArgv", err)
	}
}
