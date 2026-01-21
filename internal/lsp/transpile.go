package lsp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/elijahmorgan/c_minus/internal/codegen"
	"github.com/elijahmorgan/c_minus/internal/parser"
	"github.com/elijahmorgan/c_minus/internal/paths"
	"github.com/elijahmorgan/c_minus/internal/project"
)

type compileCommand struct {
	Directory string   `json:"directory"`
	File      string   `json:"file"`
	Arguments []string `json:"arguments"`
}

func transpileWorkspace(proj *project.Project, openDocs map[string]string) (string, error) {
	buildDir := filepath.Join(proj.RootPath, ".c_minus")
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		return "", err
	}

	var cmds []compileCommand

	for _, mod := range proj.Modules {
		parsedFiles := make([]*parser.File, 0, len(mod.Files))
		for _, filePath := range mod.Files {
			var f *parser.File
			var err error
			if content, ok := openDocs[filePath]; ok {
				f, err = parser.ParseSource(content, filePath)
			} else {
				f, err = parser.ParseFile(filePath)
			}
			if err != nil {
				return "", fmt.Errorf("failed to parse %s: %w", filePath, err)
			}
			parsedFiles = append(parsedFiles, f)

			cFilePath := paths.ModuleCFilePath(buildDir, mod.ImportPath, filepath.Base(filePath))
			cmds = append(cmds, compileCommand{
				Directory: buildDir,
				File:      cFilePath,
				Arguments: []string{"cc", "-c", cFilePath, "-I", buildDir},
			})
		}

		if err := codegen.GenerateModule(mod, parsedFiles, buildDir); err != nil {
			return "", fmt.Errorf("failed to generate code for module %s: %w", mod.ImportPath, err)
		}
	}

	b, err := json.MarshalIndent(cmds, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(buildDir, "compile_commands.json"), b, 0644); err != nil {
		return "", err
	}

	return buildDir, nil
}
