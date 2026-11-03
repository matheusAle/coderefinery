package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Case is one (tool, fixture, variant) row of the matrix.
type Case struct {
	Tool    string // ubgate subcommand: crap, dry, mutate
	Fixture string
	Variant string
	Args    []string
}

func (c Case) Name() string { return c.Tool + "/" + c.Fixture + "/" + c.Variant }

var upstreamNames = map[string]string{"crap": "crapper", "dry": "dryer", "mutate": "mutator"}

// loadMatrix reads the matrix file. Syntax, one entry per line:
//
//	fixtures <name> <name> ...        names that "*" expands to
//	<tool> <fixture|*> <variant> [args...]   args are shell-quoted words
//
// Blank lines and lines starting with '#' are ignored.
func loadMatrix(path string) ([]Case, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var fixtures []string
	var cases []Case
	sc := bufio.NewScanner(f)
	line := 0
	for sc.Scan() {
		line++
		text := strings.TrimSpace(sc.Text())
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		words, err := splitWords(text)
		if err != nil {
			return nil, fmt.Errorf("%s:%d: %v", path, line, err)
		}
		if words[0] == "fixtures" {
			fixtures = append(fixtures, words[1:]...)
			continue
		}
		if len(words) < 3 {
			return nil, fmt.Errorf("%s:%d: want <tool> <fixture> <variant> [args...]", path, line)
		}
		if _, ok := upstreamNames[words[0]]; !ok {
			return nil, fmt.Errorf("%s:%d: unknown tool %q (want crap, dry or mutate)", path, line, words[0])
		}
		targets := []string{words[1]}
		if words[1] == "*" {
			if len(fixtures) == 0 {
				return nil, fmt.Errorf("%s:%d: '*' used before a fixtures line", path, line)
			}
			targets = fixtures
		}
		for _, fx := range targets {
			cases = append(cases, Case{Tool: words[0], Fixture: fx, Variant: words[2], Args: words[3:]})
		}
	}
	return cases, sc.Err()
}

// splitWords splits a line into shell-like words: whitespace separated,
// with '...' and "..." quoting and backslash escapes outside single quotes.
func splitWords(s string) ([]string, error) {
	var words []string
	var cur strings.Builder
	inWord := false
	var quote rune
	escaped := false
	for _, r := range s {
		switch {
		case escaped:
			cur.WriteRune(r)
			escaped = false
		case quote == '\'':
			if r == '\'' {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case quote == '"':
			if r == '"' {
				quote = 0
			} else if r == '\\' {
				escaped = true
			} else {
				cur.WriteRune(r)
			}
		case r == '\\':
			escaped = true
			inWord = true
		case r == '\'' || r == '"':
			quote = r
			inWord = true
		case r == ' ' || r == '\t':
			if inWord {
				words = append(words, cur.String())
				cur.Reset()
				inWord = false
			}
		default:
			cur.WriteRune(r)
			inWord = true
		}
	}
	if quote != 0 || escaped {
		return nil, fmt.Errorf("unterminated quote or escape in %q", s)
	}
	if inWord {
		words = append(words, cur.String())
	}
	return words, nil
}

// expandTemplate turns a command template into argv. A word that is exactly
// {args} becomes the case's arguments; {tool}, {upstream_tool}, {upstream},
// {ubmode}, {fixture} and {variant} are substituted inside any word.
func expandTemplate(tmpl string, c Case, ubmode string) ([]string, error) {
	words, err := splitWords(tmpl)
	if err != nil {
		return nil, err
	}
	repl := strings.NewReplacer(
		"{tool}", c.Tool,
		"{upstream_tool}", upstreamNames[c.Tool],
		"{upstream}", ubmode+"/upstream",
		"{ubmode}", ubmode,
		"{fixture}", c.Fixture,
		"{variant}", c.Variant,
	)
	var argv []string
	for _, w := range words {
		if w == "{args}" {
			argv = append(argv, c.Args...)
			continue
		}
		if strings.Contains(w, "{args}") {
			return nil, fmt.Errorf("{args} must be a word of its own in %q", tmpl)
		}
		argv = append(argv, repl.Replace(w))
	}
	if len(argv) == 0 {
		return nil, fmt.Errorf("empty command template")
	}
	return argv, nil
}

func filterCases(cases []Case, tools, fixtures, variants string) []Case {
	match := func(list, v string) bool {
		if list == "" || list == "all" {
			return true
		}
		for _, item := range strings.Split(list, ",") {
			if strings.TrimSpace(item) == v {
				return true
			}
		}
		return false
	}
	var out []Case
	for _, c := range cases {
		if match(tools, c.Tool) && match(fixtures, c.Fixture) && match(variants, c.Variant) {
			out = append(out, c)
		}
	}
	return out
}
