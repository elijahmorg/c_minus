package lsp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/elijahmorgan/c_minus/internal/project"
)

func TestCMHoverDataAvailableForSample2TicketCreate(t *testing.T) {
	pkgDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	repoRoot := filepath.Clean(filepath.Join(pkgDir, "..", ".."))
	sample2Root := filepath.Join(repoRoot, "sample2")

	proj, err := project.Discover(sample2Root)
	if err != nil {
		t.Fatalf("discover sample2: %v", err)
	}

	idx, err := buildModuleIndex(proj, nil)
	if err != nil {
		t.Fatalf("buildModuleIndex: %v", err)
	}

	syms := idx.Modules["ticket"]
	found := false
	for _, s := range syms {
		if s.Name == "create_ticket" {
			if !s.Public {
				t.Fatalf("expected create_ticket to be public")
			}
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected to find ticket.create_ticket in module index")
	}

	mainPath := filepath.Join(sample2Root, "main.cm")
	mainText, err := os.ReadFile(mainPath)
	if err != nil {
		t.Fatalf("read main.cm: %v", err)
	}

	imports := importedModulePrefixes(mainPath, string(mainText))
	if imports["ticket"] != "ticket" {
		t.Fatalf("expected import prefix ticket -> ticket, got %q", imports["ticket"])
	}
}
