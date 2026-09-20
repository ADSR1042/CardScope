//go:build linux

package collect

import (
	"testing"

	"github.com/ebitengine/purego"
)

// Exercise the same dynamic loader and typed function binding used by NVML,
// even on CI hosts without an NVIDIA driver.
func TestDynamicBinding(t *testing.T) {
	lib, err := purego.Dlopen("libc.so.6", purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err != nil {
		t.Fatal(err)
	}
	defer purego.Dlclose(lib)
	strlen, err := bind[func(string) uintptr](lib, "strlen")
	if err != nil {
		t.Fatal(err)
	}
	if got := strlen("CardScope"); got != 9 {
		t.Fatalf("C string binding returned %d", got)
	}
	if _, err := bind[func() int32](lib, "cardscope_missing_symbol"); err == nil {
		t.Fatal("missing symbol must return an error instead of panicking")
	}
}
