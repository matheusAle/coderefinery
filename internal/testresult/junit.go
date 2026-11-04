package testresult

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
)

// node is a generic XML element.
type node struct {
	Name     string
	Attrs    map[string]string
	Text     strings.Builder
	Children []*node
}

func (n *node) attr(k string) string { return n.Attrs[k] }

func (n *node) has(k string) bool { _, ok := n.Attrs[k]; return ok }

func (n *node) child(name string) *node {
	for _, c := range n.Children {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func parseXML(r io.Reader) (*node, error) {
	dec := xml.NewDecoder(r)
	dec.Strict = false
	dec.CharsetReader = func(_ string, in io.Reader) (io.Reader, error) { return in, nil }
	var stack []*node
	var root *node
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("invalid JUnit XML: %v", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			n := &node{Name: t.Name.Local, Attrs: map[string]string{}}
			for _, a := range t.Attr {
				n.Attrs[a.Name.Local] = a.Value
			}
			if len(stack) > 0 {
				p := stack[len(stack)-1]
				p.Children = append(p.Children, n)
			} else if root == nil {
				root = n
			}
			stack = append(stack, n)
		case xml.EndElement:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		case xml.CharData:
			if len(stack) > 0 {
				stack[len(stack)-1].Text.Write(t)
			}
		}
	}
	if root == nil {
		return nil, errors.New("invalid JUnit XML: no root element")
	}
	if root.Name != "testsuites" && root.Name != "testsuite" {
		return nil, fmt.Errorf("invalid JUnit XML: root element <%s>", root.Name)
	}
	return root, nil
}

// sniff is the tool from the root element (toolHint wins over it).
func sniff(root *node) string {
	name := root.attr("name")
	switch {
	case name == "pytest tests" || root.Name == "testsuite" && name == "pytest":
		return "pytest"
	case name == "vitest tests":
		return "vitest"
	case name == "jest tests":
		return "jest"
	case name == "Mocha Tests":
		return "mocha"
	case name == "nextest-run":
		return "nextest"
	}
	suites := []*node{root}
	if root.Name == "testsuites" {
		suites = root.Children
	}
	for _, s := range suites {
		if strings.Contains(s.attr("noNamespaceSchemaLocation"), "surefire") {
			return "surefire"
		}
		if s.Name == "testsuite" && s.child("testcase") != nil && s.child("testcase").attr("name") != "" {
			if strings.HasSuffix(s.child("testcase").attr("name"), ")") && s.has("hostname") && !s.has("file") {
				return "gradle"
			}
		}
		if s.Name == "testsuite" && s.child("testcase") != nil {
			if n := s.child("testcase").attr("name"); strings.Contains(n, "/") && strings.HasPrefix(n, s.child("testcase").attr("classname")+"/") {
				return "kaocha"
			}
		}
	}
	return "junit"
}

// testcase is one <testcase> with the attributes of its enclosing suites.
type testcase struct {
	n      *node
	suites []*node // outermost first, excluding the <testsuites> root
}

func collect(n *node, suites []*node, out *[]testcase) {
	for _, c := range n.Children {
		switch c.Name {
		case "testcase":
			*out = append(*out, testcase{c, suites})
		case "testsuite":
			collect(c, append(append([]*node(nil), suites...), c), out)
		}
	}
}

func seconds(s string) float64 {
	f, err := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(s), ",", ""), 64)
	if err != nil || f < 0 {
		return 0
	}
	return f
}

// outcome reads a testcase's status and failure text.
func outcome(n *node) (Status, string) {
	for _, tag := range []string{"error", "failure"} {
		var parts []string
		found := false
		for _, c := range n.Children {
			if c.Name != tag {
				continue
			}
			found = true
			if m := strings.TrimSpace(c.attr("message")); m != "" {
				parts = append(parts, m)
			}
			if t := strings.TrimSpace(c.Text.String()); t != "" && (len(parts) == 0 || !strings.Contains(t, parts[len(parts)-1]) || len(t) > len(parts[len(parts)-1])) {
				parts = append(parts, t)
			}
		}
		if found {
			st := Fail
			if tag == "error" {
				st = Error
			}
			return st, strings.Join(parts, "\n")
		}
	}
	if n.child("skipped") != nil {
		return Skip, ""
	}
	return Pass, ""
}

// ParseJUnit reads a JUnit XML report. root is the directory the tool ran
// in: relative paths in the report resolve against it, and the IDs use paths
// relative to it (Run.Rebase makes them repo-relative).
func ParseJUnit(r io.Reader, root, toolHint string) (*Run, error) {
	doc, err := parseXML(r)
	if err != nil {
		return nil, err
	}
	tool := toolHint
	if tool == "" {
		tool = sniff(doc)
	}
	var cases []testcase
	if doc.Name == "testsuite" {
		collect(&node{Children: []*node{doc}}, nil, &cases)
	} else {
		collect(doc, nil, &cases)
	}
	p := &junitParser{root: root, tool: tool, res: newResolver(root)}
	run := &Run{Tool: tool}
	for _, tc := range cases {
		c, buildFailure := p.convert(tc)
		if buildFailure {
			run.BuildFailed = true
			continue
		}
		run.Cases = append(run.Cases, c)
	}
	run.normalize()
	return run, nil
}

type junitParser struct {
	root, tool string
	res        *resolver
}

var (
	pytestParam  = regexp.MustCompile(`\[.*\]$`)
	javaParam    = regexp.MustCompile(`(\(.*\))?\[\d+\]$`)
	rstestCase   = regexp.MustCompile(`::case_\d+.*$`)
	gradleParens = regexp.MustCompile(`\(\)$`)
)

// convert maps one testcase; buildFailure=true means it is a collection /
// suite-load failure that is not tied to a case.
func (p *junitParser) convert(tc testcase) (Case, bool) {
	n := tc.n
	status, failure := outcome(n)
	c := Case{Status: status, Failure: failure, Duration: seconds(n.attr("time"))}
	name, classname := n.attr("name"), n.attr("classname")
	baseName := "" // Name minus the tool's parameter part; "" = Name
	switch p.tool {
	case "pytest":
		if classname == "" && status == Error && strings.Contains(failure, "collection failure") || n.child("error") != nil && n.child("error").attr("message") == "collection failure" {
			return c, true
		}
		file := p.res.rel(n.attr("file"))
		modParts := []string{}
		if file != "" {
			modParts = strings.Split(strings.TrimSuffix(file, ".py"), "/")
		} else if f := p.res.pythonModule(classname); f != "" {
			file = f
			modParts = strings.Split(strings.TrimSuffix(f, ".py"), "/")
		}
		c.File, c.Locator = file, file
		if file == "" {
			c.Locator = classname
		} else {
			c.Suite = pytestSuite(classname, modParts)
		}
		c.Name = name
		baseName = pytestParam.ReplaceAllString(name, "")
	case "vitest":
		if name == classname && status != Pass && status != Skip {
			return c, true // the file failed to load
		}
		c.File = p.res.rel(classname)
		c.Locator = c.File
		parts := strings.Split(name, " > ")
		c.Suite, c.Name = parts[:len(parts)-1], parts[len(parts)-1]
	case "jest":
		if classname == "Test suite failed to run" {
			return c, true
		}
		c.File = p.res.rel(n.attr("file"))
		c.Locator = c.File
		// The wrapper sets JEST_JUNIT_CLASSNAME={classname}, JEST_JUNIT_TITLE={title}
		// and the " › " separator. With jest-junit's default templates both
		// attributes are "{classname} {title}" and the split is unknowable:
		// the whole name is kept.
		if classname != "" && classname != name {
			c.Suite = splitTrim(classname, "›")
		}
		c.Name = name
	case "mocha":
		var file, suiteName string
		for _, s := range tc.suites {
			if f := s.attr("file"); f != "" {
				file = f
			}
			suiteName = s.attr("name")
		}
		c.File = p.res.rel(file)
		c.Locator = c.File
		title := classname
		if strings.Contains(suiteName, "›") || strings.HasPrefix(suiteName, "Root Suite") {
			parts := splitTrim(strings.TrimPrefix(suiteName, "Root Suite"), "›")
			c.Suite = parts
		} else if prefix := strings.TrimSuffix(name, " "+title); prefix != name && prefix != "" {
			c.Suite = []string{prefix}
		}
		if title == "" {
			title = name
		}
		c.Name = title
	case "surefire", "gradle":
		fqcn := classname
		outer, inner := fqcn, []string(nil)
		if i := strings.IndexByte(fqcn, '$'); i >= 0 {
			outer = fqcn[:i]
			inner = strings.Split(fqcn[i+1:], "$")
		}
		simple := outer[strings.LastIndexByte(outer, '.')+1:]
		file := p.res.javaFile(outer)
		c.File = file
		if file != "" {
			c.Locator = file
			c.Suite = append([]string{simple}, inner...)
		} else {
			c.Locator = outer
			c.Suite = inner
		}
		c.Name = gradleParens.ReplaceAllString(name, "")
		baseName = javaParam.ReplaceAllString(c.Name, "")
	case "nextest":
		binary := classname
		if binary == "" && len(tc.suites) > 0 {
			binary = tc.suites[len(tc.suites)-1].attr("name")
		}
		c.Locator = binary
		c.File = p.res.rustTestFile(binary)
		if c.File != "" {
			c.Locator = c.File
		}
		c.Name = name
		baseName = rstestCase.ReplaceAllString(name, "")
	case "kaocha", "test2junit":
		ns := classname
		if ns == "" {
			if i := strings.LastIndexByte(name, '/'); i > 0 {
				ns = name[:i]
			}
		}
		varName := strings.TrimPrefix(name, ns+"/")
		c.File = p.res.clojureFile(ns)
		c.Locator = c.File
		if c.File == "" {
			c.Locator = ns
		}
		c.Name = varName
	default: // generic JUnit
		file := p.res.rel(n.attr("file"))
		if file == "" && len(tc.suites) > 0 {
			file = p.res.rel(tc.suites[len(tc.suites)-1].attr("file"))
		}
		c.File = file
		c.Locator = file
		if file == "" {
			c.Locator = classname
		} else if classname != "" && classname != name {
			c.Suite = []string{classname}
		}
		c.Name = name
	}
	if baseName == "" {
		baseName = c.Name
	}
	c.finish(baseName)
	return c, false
}

// pytestSuite is the classname minus the module's dotted path: the longest
// run of the file's trailing module parts that prefixes the classname.
func pytestSuite(classname string, modParts []string) []string {
	cls := strings.Split(classname, ".")
	best := 0
	for k := 1; k <= len(modParts) && k <= len(cls); k++ {
		if strings.Join(cls[:k], ".") == strings.Join(modParts[len(modParts)-k:], ".") {
			best = k
		}
	}
	if best == 0 {
		return nil
	}
	return cls[best:]
}

func splitTrim(s, sep string) []string {
	var out []string
	for _, p := range strings.Split(s, sep) {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
