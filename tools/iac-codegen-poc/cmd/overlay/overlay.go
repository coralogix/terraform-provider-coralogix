package main

import (
	"bytes"
	"fmt"
	"strings"

	"go.yaml.in/yaml/v4"
)

// An overlay file is a YAML list of entries. The entries are applied in order.
//
//	- path: components.schemas.SqlLoadConfig.properties.joinLimit.format
//	  set: uint64
//	- path: components.schemas.SqlLoadConfig.properties.cteLimit
//	  remove: true
//
// set adds the last key of the path, or replaces its value. All keys before the
// last one must exist. remove deletes the last key of the path. The key must exist.
//
// Path syntax: keys are separated by ".". A key may contain any character except
// "." and "[", for example "/ai/evaluations/v3/{id}" or "application/json".
// Put a key that contains "." in brackets:
//
//	components.schemas[v3.FilterOperator].type
//
// Paths go through mappings only. List indexes are not supported.
//
// The output is the source text with only the changed lines replaced. Every other
// line stays byte for byte. After each entry the result is parsed again and compared
// with the same edit made on the node tree. A mismatch is an error.

type entry struct {
	index  int // 1-based position in the overlay list
	line   int // line in the overlay file
	path   string
	keys   []string
	set    *yaml.Node // nil when remove is true
	remove bool
}

func (e entry) errorf(format string, args ...any) error {
	return fmt.Errorf("entry %d (line %d, path %q): %s", e.index, e.line, e.path, fmt.Sprintf(format, args...))
}

// dumpOptions render new values in the style of the source spec:
// 2-space indent, list items indented under their key, single quotes.
var dumpOptions = []yaml.Option{
	yaml.WithIndent(2),
	yaml.WithCompactSeqIndent(false),
	yaml.WithQuotePreference(yaml.QuoteSingle),
}

func parseEntries(data []byte) ([]entry, error) {
	var doc yaml.Node
	if err := yaml.Load(data, &doc); err != nil {
		return nil, err
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("the overlay must be a list of entries")
	}
	var entries []entry
	for i, item := range doc.Content[0].Content {
		e, err := parseEntry(i+1, item)
		if err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, nil
}

func parseEntry(index int, item *yaml.Node) (entry, error) {
	e := entry{index: index, line: item.Line}
	if item.Kind != yaml.MappingNode {
		return e, e.errorf("an entry must be a mapping with path and set, or path and remove")
	}
	for i := 0; i < len(item.Content); i += 2 {
		key, value := item.Content[i], item.Content[i+1]
		switch key.Value {
		case "path":
			if value.Kind != yaml.ScalarNode || value.ShortTag() != "!!str" {
				return e, e.errorf("path must be a string")
			}
			e.path = value.Value
		case "set":
			e.set = value
		case "remove":
			if value.ShortTag() != "!!bool" || value.Value != "true" {
				return e, e.errorf("remove must be true")
			}
			e.remove = true
		default:
			return e, e.errorf("unknown field %q (want path, set, or remove)", key.Value)
		}
	}
	if e.path == "" {
		return e, e.errorf("path is missing")
	}
	if (e.set == nil) == !e.remove {
		return e, e.errorf("use exactly one of set and remove")
	}
	keys, err := parsePath(e.path)
	if err != nil {
		return e, e.errorf("%v", err)
	}
	e.keys = keys
	return e, nil
}

func parsePath(path string) ([]string, error) {
	var keys []string
	for i := 0; i < len(path); {
		var key string
		if path[i] == '[' {
			end := strings.IndexByte(path[i:], ']')
			if end < 0 {
				return nil, fmt.Errorf("missing ] after offset %d", i)
			}
			key = path[i+1 : i+end]
			i += end + 1
			if i < len(path) && path[i] != '.' && path[i] != '[' {
				return nil, fmt.Errorf("want . or [ after ] at offset %d", i)
			}
		} else {
			end := strings.IndexAny(path[i:], ".[")
			if end < 0 {
				end = len(path) - i
			}
			key = path[i : i+end]
			i += end
		}
		if key == "" {
			return nil, fmt.Errorf("empty key at offset %d", i)
		}
		keys = append(keys, key)
		if i < len(path) && path[i] == '.' {
			i++
			if i == len(path) {
				return nil, fmt.Errorf("path ends with .")
			}
		}
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("path is empty")
	}
	return keys, nil
}

func formatPath(keys []string) string {
	var b strings.Builder
	for i, k := range keys {
		if strings.ContainsAny(k, ".[") {
			b.WriteString("[" + k + "]")
			continue
		}
		if i > 0 {
			b.WriteByte('.')
		}
		b.WriteString(k)
	}
	return b.String()
}

// apply returns src with the entries applied in order.
func apply(src []byte, entries []entry) ([]byte, error) {
	if len(src) > 0 && src[len(src)-1] != '\n' {
		return nil, fmt.Errorf("the source must end with a newline")
	}
	text := src
	current, err := parseRoot(text)
	if err != nil {
		return nil, err
	}
	// want gets the same edits as the text, made directly on the nodes.
	want, err := parseRoot(text)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if text, err = e.applyText(text, current); err != nil {
			return nil, err
		}
		if err := e.applyNode(want); err != nil {
			return nil, err
		}
		if current, err = parseRoot(text); err != nil {
			return nil, e.errorf("the result is not valid YAML: %v", err)
		}
		if at, differ := firstDiff(current, want, nil); differ {
			return nil, e.errorf("internal error: the text edit does not match the node edit at %q", at)
		}
	}
	return text, nil
}

func parseRoot(text []byte) (*yaml.Node, error) {
	var doc yaml.Node
	if err := yaml.Load(text, &doc); err != nil {
		return nil, err
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("the document root must be a mapping")
	}
	return doc.Content[0], nil
}

// frame is one mapping on the way to the target key, and the index of the key
// that the path follows in it.
type frame struct {
	mapping *yaml.Node
	keyIdx  int
}

// locate returns the mappings on the path to the parent of the last key, the
// parent mapping, and the index of the last key in it (-1 when it is missing).
func (e entry) locate(root *yaml.Node) ([]frame, *yaml.Node, int, error) {
	var frames []frame
	node := root
	for depth, key := range e.keys {
		if node.Kind != yaml.MappingNode {
			return nil, nil, 0, e.errorf("%q is not a mapping", formatPath(e.keys[:depth]))
		}
		i := findKey(node, key)
		if depth == len(e.keys)-1 {
			return frames, node, i, nil
		}
		if i < 0 {
			return nil, nil, 0, e.errorf("key %q not found in %q", key, formatPath(e.keys[:depth]))
		}
		frames = append(frames, frame{node, i})
		node = node.Content[i+1]
	}
	panic("unreachable: keys is never empty")
}

func findKey(mapping *yaml.Node, key string) int {
	for i := 0; i < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return i
		}
	}
	return -1
}

func (e entry) applyNode(root *yaml.Node) error {
	_, parent, i, err := e.locate(root)
	if err != nil {
		return err
	}
	switch {
	case e.remove:
		parent.Content = append(parent.Content[:i:i], parent.Content[i+2:]...)
	case i >= 0:
		parent.Content[i+1] = e.set
	default:
		key := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: e.keys[len(e.keys)-1]}
		parent.Content = append(parent.Content, key, e.set)
	}
	return nil
}

func (e entry) applyText(text []byte, root *yaml.Node) ([]byte, error) {
	frames, parent, i, err := e.locate(root)
	if err != nil {
		return nil, err
	}
	if parent.Style&yaml.FlowStyle != 0 {
		return nil, e.errorf("the parent mapping uses flow style ({...}); only block style is supported")
	}
	lines := bytes.SplitAfter(text, []byte("\n"))
	lines = lines[:len(lines)-1] // text ends with "\n", so the last element is empty
	key := e.keys[len(e.keys)-1]

	if i < 0 {
		if e.remove {
			return nil, e.errorf("key %q not found in %q, nothing to remove", key, formatPath(e.keys[:len(e.keys)-1]))
		}
		// Add the key after the last key of the parent, at the same indent.
		last := parent.Content[len(parent.Content)-2]
		at := blockEnd(lines, frames, last.Line)
		rendered, err := e.render(key, last.Column-1)
		if err != nil {
			return nil, err
		}
		return splice(lines, at, at, rendered), nil
	}

	keyNode := parent.Content[i]
	if !isIndent(lines[keyNode.Line-1][:keyNode.Column-1]) {
		return nil, e.errorf("key %q does not start its line; this layout is not supported", key)
	}
	end := blockEnd(lines, append(frames, frame{parent, i}), keyNode.Line)
	if e.remove {
		if len(parent.Content) == 2 {
			return nil, e.errorf("remove would leave %q empty", formatPath(e.keys[:len(e.keys)-1]))
		}
		return splice(lines, keyNode.Line, end, nil), nil
	}
	rendered, err := e.render(key, keyNode.Column-1)
	if err != nil {
		return nil, err
	}
	return splice(lines, keyNode.Line, end, rendered), nil
}

// blockEnd returns the first line after the block of the key that the last frame
// points to. start is the first line of that block. Trailing blank lines are not
// part of the block.
func blockEnd(lines [][]byte, frames []frame, start int) int {
	end := len(lines) + 1
	for f := len(frames) - 1; f >= 0; f-- {
		m, i := frames[f].mapping, frames[f].keyIdx
		if i+2 < len(m.Content) {
			end = m.Content[i+2].Line
			break
		}
	}
	for end-1 > start && len(bytes.TrimSpace(lines[end-2])) == 0 {
		end--
	}
	return end
}

// splice replaces lines [from, to) (1-based) with insert.
func splice(lines [][]byte, from, to int, insert []byte) []byte {
	var out bytes.Buffer
	for _, l := range lines[:from-1] {
		out.Write(l)
	}
	out.Write(insert)
	for _, l := range lines[to-1:] {
		out.Write(l)
	}
	return out.Bytes()
}

// render returns "key: value" as block YAML, indented by indent spaces.
func (e entry) render(key string, indent int) ([]byte, error) {
	m := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Content: []*yaml.Node{
		{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
		plain(e.set),
	}}
	out, err := yaml.Dump(m, dumpOptions...)
	if err != nil {
		return nil, e.errorf("render value: %v", err)
	}
	pad := []byte(strings.Repeat(" ", indent))
	var b bytes.Buffer
	for _, l := range bytes.SplitAfter(out, []byte("\n")) {
		if len(l) > 0 {
			b.Write(pad)
			b.Write(l)
		}
	}
	return b.Bytes(), nil
}

// plain returns a copy of n without styles and comments, so the value is
// written in block style like the rest of the spec, not as written in the overlay.
func plain(n *yaml.Node) *yaml.Node {
	c := &yaml.Node{Kind: n.Kind, Tag: n.Tag, Value: n.Value}
	for _, child := range n.Content {
		c.Content = append(c.Content, plain(child))
	}
	return c
}

func isIndent(b []byte) bool {
	return len(bytes.Trim(b, " ")) == 0
}

// firstDiff returns the path of the first difference between a and b.
// Styles, comments, and positions are ignored.
func firstDiff(a, b *yaml.Node, keys []string) (string, bool) {
	if a.Kind != b.Kind || a.ShortTag() != b.ShortTag() || a.Value != b.Value || len(a.Content) != len(b.Content) {
		return formatPath(keys), true
	}
	for i := range a.Content {
		next := keys
		if a.Kind == yaml.MappingNode {
			next = append(keys[:len(keys):len(keys)], a.Content[i-i%2].Value)
		}
		if at, differ := firstDiff(a.Content[i], b.Content[i], next); differ {
			return at, true
		}
	}
	return "", false
}
