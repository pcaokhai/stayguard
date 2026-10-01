package pricing

import (
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"strings"
)

// Stable reason codes; the HTTP layer maps them to messages later.
const (
	CodeInvalidJSON     = "INVALID_JSON"
	CodeRequired        = "REQUIRED"
	CodeType            = "TYPE"
	CodeMin             = "MIN"
	CodeMax             = "MAX"
	CodePattern         = "PATTERN"
	CodeUnknownProperty = "UNKNOWN_PROPERTY"
	CodeConst           = "CONST"
)

// FieldError never carries the offending value: it can quote client data.
type FieldError struct {
	Path string
	Code string
}

type ValidationError struct {
	Errors []FieldError
}

func (e *ValidationError) Error() string {
	parts := make([]string, len(e.Errors))
	for i, fe := range e.Errors {
		parts[i] = fe.Path + " " + fe.Code
	}
	return "invalid rate plan: " + strings.Join(parts, "; ")
}

// collector gathers every error instead of stopping at the first.
type collector struct{ errs []FieldError }

func (c *collector) add(path, code string) { c.errs = append(c.errs, FieldError{path, code}) }

func (c *collector) sorted() []FieldError {
	sort.Slice(c.errs, func(i, j int) bool {
		a, b := c.errs[i], c.errs[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		return a.Code < b.Code
	})
	return c.errs
}

func join(parent, key string) string {
	if parent == "" {
		return key
	}
	return parent + "." + key
}

// object checks the value is an object with exactly the allowed keys, all required.
func (c *collector) object(path string, v any, keys ...string) map[string]any {
	m, ok := v.(map[string]any)
	if !ok {
		c.add(path, CodeType)
		return nil
	}
	allowed := make(map[string]bool, len(keys))
	for _, k := range keys {
		allowed[k] = true
		if _, present := m[k]; !present {
			c.add(join(path, k), CodeRequired)
		}
	}
	// One error per object, at the parent path: key names are client data and are never echoed.
	for k := range m {
		if !allowed[k] {
			c.add(path, CodeUnknownProperty)
			break
		}
	}
	return m
}

// integer reads a plain integer literal (no fraction or exponent) within [lo, hi].
// An absent key was already reported as REQUIRED, so it returns zero silently.
func (c *collector) integer(path string, m map[string]any, key string, lo, hi int64) int64 {
	raw, present := m[key]
	if !present {
		return 0
	}
	p := join(path, key)
	n, ok := raw.(json.Number)
	if !ok {
		c.add(p, CodeType)
		return 0
	}
	v, err := strconv.ParseInt(n.String(), 10, 64)
	switch {
	case errors.Is(err, strconv.ErrRange):
		// Beyond int64 is a bound violation, not a type error; the schema would accept it.
		c.add(p, rangeCode(n))
	case err != nil:
		c.add(p, CodeType)
	case v < lo:
		c.add(p, CodeMin)
	case v > hi:
		c.add(p, CodeMax)
	default:
		return v
	}
	return 0
}

func rangeCode(n json.Number) string {
	if strings.HasPrefix(n.String(), "-") {
		return CodeMin
	}
	return CodeMax
}

func (c *collector) clock(path string, m map[string]any, key string) Clock {
	raw, present := m[key]
	if !present {
		return Clock{}
	}
	p := join(path, key)
	s, ok := raw.(string)
	if !ok {
		c.add(p, CodeType)
		return Clock{}
	}
	clk, ok := parseClock(s)
	if !ok {
		c.add(p, CodePattern)
	}
	return clk
}
