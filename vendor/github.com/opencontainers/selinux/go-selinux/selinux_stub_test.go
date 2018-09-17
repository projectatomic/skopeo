// +build !selinux

package selinux

import (
	"testing"
)

func TestSELinux(t *testing.T) {
	if GetEnabled() {
		t.Fatal("SELinux enabled with build-tag !selinux.")
	}
}
