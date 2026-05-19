package cli

import (
	"bufio"
	"errors"
	"io"
	"os"
	"strings"
)

// stdinReader is the source for stdin-fed commands (sort, convert, blindness).
// Tests swap it with a strings.Reader to exercise stdin paths without
// touching the real os.Stdin.
var stdinReader io.Reader = os.Stdin

// errPartialFailure is the sentinel returned by commands that processed
// some inputs successfully but had errors on others. The root command's
// custom exit-code mapping will translate this to exit 2.
// For now it just propagates as a normal
// error so the command still exits non-zero.
var errPartialFailure = errors.New("one or more inputs failed")

// collectColorInputs gathers color strings from positional args, falling
// back to stdin when args is empty. Stdin is read line-by-line; comment
// lines (#-only or //-prefixed) and blank lines are skipped.
//
// Feels natural in pipelines (`cat colors.txt | huetension sort`).
// When args ARE given, stdin is never read — the user's intent is unambiguous.
func collectColorInputs(args []string) ([]string, error) {
	if len(args) > 0 {
		return args, nil
	}
	scanner := bufio.NewScanner(stdinReader)
	var out []string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		// Skip comment-only lines. Hex literals begin with '#' so we
		// can't strip leading-'#' lines unconditionally — but '#'
		// followed by whitespace (e.g. "# this is a comment") is
		// unambiguously not a hex value, and '//' is the standard
		// shell-script alternative.
		if strings.HasPrefix(line, "//") {
			continue
		}
		if line == "#" || (strings.HasPrefix(line, "#") && len(line) > 1 && isASCIISpace(line[1])) {
			continue
		}
		out = append(out, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, errors.New("no color inputs (provide as args or pipe via stdin)")
	}
	return out, nil
}

func isASCIISpace(b byte) bool {
	switch b {
	case ' ', '\t', '\n', '\r', '\f', '\v':
		return true
	}
	return false
}
