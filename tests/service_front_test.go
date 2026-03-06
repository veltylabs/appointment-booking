//go:build wasm

package tests

import (
	"testing"
)

func TestService_Pure(t *testing.T) {
	// For WASM testing of the pure logic, we rely on the Stlib (!wasm) backend tests
	// as tinywasm/sqlite currently imports CGO/native bindings that block WASM compilation.
}
