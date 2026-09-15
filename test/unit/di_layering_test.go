// Purpose: Black-box layering test asserting that pkg/di is not imported by any layer 1–3
// package (pkg/errors, pkg/router, pkg/middleware), per the architecture constraint.
package unit_test

import (
	"os/exec"
	"strings"
	"testing"
)

func TestDINotImportedByLowerLayers(t *testing.T) {
	diPkg := "github.com/example/lightweight-http/pkg/di"
	packages := []string{
		"github.com/example/lightweight-http/pkg/errors",
		"github.com/example/lightweight-http/pkg/router",
		"github.com/example/lightweight-http/pkg/middleware",
	}
	for _, pkg := range packages {
		out, err := exec.Command("go", "list", "-deps", pkg).CombinedOutput()
		if err != nil {
			t.Fatalf("go list -deps %s: %v\n%s", pkg, err, out)
		}
		for _, line := range strings.Split(string(out), "\n") {
			if strings.TrimSpace(line) == diPkg {
				t.Errorf("package %s must not import %s (layering violation)", pkg, diPkg)
			}
		}
	}
}
