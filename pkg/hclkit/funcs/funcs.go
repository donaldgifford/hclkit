// Package funcs provides the standard hclkit function bundle: env,
// the four case-conversion helpers lifted from forge, and now.
// Consumers opt in via hclkit's EvalCtxBuilder.WithStdFuncs or by
// merging Std() into their own function map.
//
// The HCL-visible names (snakeCase, camelCase, pascalCase, kebabCase,
// now, env) are config-file surface and frozen at v0. They keep
// forge's camelCase convention — do not "fix" them to Terraform-style
// snake_case.
//
// This package must import only hcl/cty and the standard library —
// never hclkit itself — so pkg/hclkit can depend on it without a
// cycle.
package funcs

import (
	"os"
	"strings"
	"time"
	"unicode"

	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
)

// Std returns the standard function bundle keyed by HCL-visible name,
// with env bound to the process environment (EnvFunc). Each call
// returns a fresh map so callers can merge or override entries
// without mutating a shared bundle.
func Std() map[string]function.Function {
	return map[string]function.Function{
		"snakeCase":  SnakeCaseFunc,
		"camelCase":  CamelCaseFunc,
		"pascalCase": PascalCaseFunc,
		"kebabCase":  KebabCaseFunc,
		"now":        NowFunc,
		"env":        EnvFunc,
	}
}

// EnvFunc is env(name) bound to the process environment — Env(nil),
// exported for symmetry with the other bundle members. Use Env for a
// custom lookup (tests, allowlists).
var EnvFunc = Env(nil)

// Env returns the env(name) function backed by lookup. A nil lookup
// uses os.Getenv. Missing keys return the empty string (Unix shell
// semantics) — this is the canonical env() that replaces
// repo-guardian's post-decode allowlist.
func Env(lookup func(string) string) function.Function {
	if lookup == nil {
		lookup = os.Getenv
	}
	return function.New(&function.Spec{
		Description: "Returns the value of the named environment variable, or empty string if unset.",
		Params: []function.Parameter{
			{Name: "name", Type: cty.String},
		},
		Type: function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, _ cty.Type) (cty.Value, error) {
			return cty.StringVal(lookup(args[0].AsString())), nil
		},
	})
}

// SnakeCaseFunc converts a string to snake_case.
var SnakeCaseFunc = function.New(&function.Spec{
	Description: "Converts a string to snake_case.",
	Params: []function.Parameter{
		{Name: "str", Type: cty.String},
	},
	Type: function.StaticReturnType(cty.String),
	Impl: func(args []cty.Value, _ cty.Type) (cty.Value, error) {
		return cty.StringVal(snakeCase(args[0].AsString())), nil
	},
})

// CamelCaseFunc converts a string to camelCase.
var CamelCaseFunc = function.New(&function.Spec{
	Description: "Converts a string to camelCase.",
	Params: []function.Parameter{
		{Name: "str", Type: cty.String},
	},
	Type: function.StaticReturnType(cty.String),
	Impl: func(args []cty.Value, _ cty.Type) (cty.Value, error) {
		return cty.StringVal(camelCase(args[0].AsString())), nil
	},
})

// PascalCaseFunc converts a string to PascalCase.
var PascalCaseFunc = function.New(&function.Spec{
	Description: "Converts a string to PascalCase.",
	Params: []function.Parameter{
		{Name: "str", Type: cty.String},
	},
	Type: function.StaticReturnType(cty.String),
	Impl: func(args []cty.Value, _ cty.Type) (cty.Value, error) {
		return cty.StringVal(pascalCase(args[0].AsString())), nil
	},
})

// KebabCaseFunc converts a string to kebab-case.
var KebabCaseFunc = function.New(&function.Spec{
	Description: "Converts a string to kebab-case.",
	Params: []function.Parameter{
		{Name: "str", Type: cty.String},
	},
	Type: function.StaticReturnType(cty.String),
	Impl: func(args []cty.Value, _ cty.Type) (cty.Value, error) {
		return cty.StringVal(kebabCase(args[0].AsString())), nil
	},
})

// NowFunc returns the current UTC time formatted with the given Go
// layout. It calls time.Now on every invocation — results are never
// memoized across loads (forge formats in local time; the hclkit
// bundle is UTC per DESIGN-0001).
var NowFunc = function.New(&function.Spec{
	Description: "Returns the current UTC time formatted with the given Go layout.",
	Params: []function.Parameter{
		{Name: "layout", Type: cty.String},
	},
	Type: function.StaticReturnType(cty.String),
	Impl: func(args []cty.Value, _ cty.Type) (cty.Value, error) {
		return cty.StringVal(time.Now().UTC().Format(args[0].AsString())), nil
	},
})

// The conversion helpers below are lifted from forge's
// internal/template/funcs.go so migrated configs render identically.

func snakeCase(s string) string {
	return toDelimited(s, '_')
}

func kebabCase(s string) string {
	return toDelimited(s, '-')
}

func camelCase(s string) string {
	words := splitWords(s)
	if len(words) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString(strings.ToLower(words[0]))
	for _, w := range words[1:] {
		b.WriteString(capitalize(w))
	}

	return b.String()
}

func pascalCase(s string) string {
	var b strings.Builder
	for _, w := range splitWords(s) {
		b.WriteString(capitalize(w))
	}

	return b.String()
}

// splitWords splits a string into words by separators and casing
// transitions. Runs of uppercase letters split at every letter
// ("HTTPServer" → H, T, T, P, Server) — forge semantics, preserved
// for 1:1 config compatibility.
func splitWords(s string) []string {
	var words []string
	current := make([]rune, 0, len(s))

	for i, r := range s {
		switch {
		case r == '_' || r == '-' || r == ' ' || r == '.':
			if len(current) > 0 {
				words = append(words, string(current))
				current = current[:0]
			}
		case unicode.IsUpper(r) && i > 0 && len(current) > 0:
			words = append(words, string(current))
			current = current[:0]
			current = append(current, r)
		default:
			current = append(current, r)
		}
	}

	if len(current) > 0 {
		words = append(words, string(current))
	}

	return words
}

func toDelimited(s string, delim rune) string {
	words := splitWords(s)
	parts := make([]string, len(words))

	for i, w := range words {
		parts[i] = strings.ToLower(w)
	}

	return strings.Join(parts, string(delim))
}

func capitalize(s string) string {
	if s == "" {
		return ""
	}

	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])

	return string(runes)
}
