package cli

import (
	"errors"
	"fmt"
	"testing"
)

func TestSetupError_Is(t *testing.T) {
	base := fmt.Errorf("ANTHROPIC_API_KEY is not set")
	se := &SetupError{Err: base}

	var target *SetupError
	if !errors.As(se, &target) {
		t.Error("errors.As should match *SetupError")
	}

	if se.Error() != base.Error() {
		t.Errorf("SetupError.Error() = %q, want %q", se.Error(), base.Error())
	}
}

func TestSetupError_Unwrap(t *testing.T) {
	inner := fmt.Errorf("inner error")
	se := &SetupError{Err: inner}

	if !errors.Is(se, inner) {
		t.Error("SetupError should unwrap to inner error")
	}
}

func TestSetupError_WrappedInFmt(t *testing.T) {
	inner := fmt.Errorf("bad config")
	se := &SetupError{Err: inner}
	wrapped := fmt.Errorf("run failed: %w", se)

	var target *SetupError
	if !errors.As(wrapped, &target) {
		t.Error("errors.As should find SetupError through fmt.Errorf wrapping")
	}
}

func TestRegularError_IsNotSetupError(t *testing.T) {
	regular := fmt.Errorf("some runtime error")

	var target *SetupError
	if errors.As(regular, &target) {
		t.Error("regular error should not match SetupError")
	}
}
