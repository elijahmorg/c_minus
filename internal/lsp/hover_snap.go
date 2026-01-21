package lsp

import (
	"bufio"
	"os"
)

func snapCharToIdentifierInCFile(cPath string, line1Based int, char0 int) (int, bool) {
	f, err := os.Open(cPath)
	if err != nil {
		return 0, false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	line := 0
	for scanner.Scan() {
		line++
		if line == line1Based {
			text := scanner.Text()
			return snapCharToIdentifier(text, char0)
		}
	}
	return 0, false
}

func snapCharToIdentifier(line string, char0 int) (int, bool) {
	if char0 < 0 {
		char0 = 0
	}
	if char0 > len(line) {
		char0 = len(line)
	}
	if len(line) == 0 {
		return 0, false
	}

	// If we're on an identifier already, keep it.
	if char0 < len(line) && isIdentChar(line[char0]) {
		return char0, true
	}
	if char0 > 0 && isIdentChar(line[char0-1]) {
		// Move left onto the identifier.
		i := char0 - 1
		for i > 0 && isIdentChar(line[i-1]) {
			i--
		}
		return i, true
	}

	// Search right for the next identifier.
	for i := char0; i < len(line); i++ {
		if isIdentChar(line[i]) {
			for i > 0 && isIdentChar(line[i-1]) {
				i--
			}
			return i, true
		}
	}

	// Search left for the previous identifier.
	for i := char0 - 1; i >= 0; i-- {
		if isIdentChar(line[i]) {
			for i > 0 && isIdentChar(line[i-1]) {
				i--
			}
			return i, true
		}
	}

	return 0, false
}
