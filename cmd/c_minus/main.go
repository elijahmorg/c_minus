package main

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/elijahmorgan/c_minus/internal/build"
	"github.com/elijahmorgan/c_minus/internal/project"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) < 2 {
		return fmt.Errorf("usage: c_minus <command> [args...]\n\nCommands:\n  build    Build the project")
	}

	cmd := os.Args[1]

	switch cmd {
	case "build":
		return runBuild()
	default:
		return fmt.Errorf("unknown command: %s", cmd)
	}
}

func runBuild() error {
	// Parse flags
	opts := build.Options{
		Jobs:       runtime.GOMAXPROCS(0),
		OutputPath: "",
	}

	// Build context for build tags
	var customTags []string
	release := false

	// Parse flags from remaining args
	args := os.Args[2:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-j":
			if i+1 >= len(args) {
				return fmt.Errorf("-j requires an argument")
			}
			if _, err := fmt.Sscanf(args[i+1], "%d", &opts.Jobs); err != nil {
				return fmt.Errorf("invalid -j value: %v", err)
			}
			i++
		case "-o":
			if i+1 >= len(args) {
				return fmt.Errorf("-o requires an argument")
			}
			opts.OutputPath = args[i+1]
			i++
		case "-tags":
			if i+1 >= len(args) {
				return fmt.Errorf("-tags requires an argument")
			}
			// Parse comma-separated tags
			tagStr := args[i+1]
			for _, tag := range strings.Split(tagStr, ",") {
				tag = strings.TrimSpace(tag)
				if tag != "" {
					customTags = append(customTags, tag)
				}
			}
			i++
		case "--release":
			release = true
		}
	}

	// Create build context
	ctx := project.NewBuildContext(customTags, release)

	// Discover project from current directory with build context
	proj, err := project.DiscoverWithContext(".", ctx)
	if err != nil {
		return fmt.Errorf("project discovery failed: %w", err)
	}

	// Build the project
	if err := build.Build(proj, opts); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	fmt.Println("Build succeeded")
	return nil
}
