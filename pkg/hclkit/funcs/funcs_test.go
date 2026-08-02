package funcs_test

import (
	"testing"
	"time"

	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"

	"github.com/donaldgifford/hclkit/pkg/hclkit/funcs"
)

func TestCaseFunctions(t *testing.T) {
	tests := []struct {
		name string
		fn   function.Function
		in   string
		want string
	}{
		{"snake from spaces", funcs.SnakeCaseFunc, "hello world", "hello_world"},
		{"snake from pascal", funcs.SnakeCaseFunc, "HelloWorld", "hello_world"},
		{"snake from kebab", funcs.SnakeCaseFunc, "hello-world", "hello_world"},
		{"snake from dotted", funcs.SnakeCaseFunc, "hello.world", "hello_world"},
		{"snake splits uppercase runs", funcs.SnakeCaseFunc, "HTTPServer", "h_t_t_p_server"},
		{"snake empty", funcs.SnakeCaseFunc, "", ""},
		{"camel from snake", funcs.CamelCaseFunc, "hello_world", "helloWorld"},
		{"camel from pascal", funcs.CamelCaseFunc, "HelloWorld", "helloWorld"},
		{"camel empty", funcs.CamelCaseFunc, "", ""},
		{"pascal from snake", funcs.PascalCaseFunc, "hello_world", "HelloWorld"},
		{"pascal from spaces", funcs.PascalCaseFunc, "hello world", "HelloWorld"},
		{"kebab from snake", funcs.KebabCaseFunc, "hello_world", "hello-world"},
		{"kebab from camel", funcs.KebabCaseFunc, "helloWorld", "hello-world"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.fn.Call([]cty.Value{cty.StringVal(tt.in)})
			if err != nil {
				t.Fatalf("Call(%q) error = %v, want nil", tt.in, err)
			}
			if got.AsString() != tt.want {
				t.Errorf("Call(%q) = %q, want %q", tt.in, got.AsString(), tt.want)
			}
		})
	}
}

func TestEnvCustomLookup(t *testing.T) {
	env := funcs.Env(func(name string) string {
		if name == "HOME" {
			return "/custom"
		}
		return ""
	})

	tests := []struct {
		name string
		key  string
		want string
	}{
		{"present key", "HOME", "/custom"},
		{"missing key returns empty", "ABSENT", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := env.Call([]cty.Value{cty.StringVal(tt.key)})
			if err != nil {
				t.Fatalf("env(%q) error = %v, want nil", tt.key, err)
			}
			if got.AsString() != tt.want {
				t.Errorf("env(%q) = %q, want %q", tt.key, got.AsString(), tt.want)
			}
		})
	}
}

func TestEnvNilLookupUsesProcessEnv(t *testing.T) {
	t.Setenv("HCLKIT_TEST_ENV", "from-process")

	got, err := funcs.Env(nil).Call([]cty.Value{cty.StringVal("HCLKIT_TEST_ENV")})
	if err != nil {
		t.Fatalf("env(HCLKIT_TEST_ENV) error = %v, want nil", err)
	}
	if got.AsString() != "from-process" {
		t.Errorf("env(HCLKIT_TEST_ENV) = %q, want %q", got.AsString(), "from-process")
	}
}

func TestEnvFuncVar(t *testing.T) {
	t.Setenv("HCLKIT_TEST_ENVFUNC", "via-var")

	got, err := funcs.EnvFunc.Call([]cty.Value{cty.StringVal("HCLKIT_TEST_ENVFUNC")})
	if err != nil {
		t.Fatalf("EnvFunc call error = %v, want nil", err)
	}
	if got.AsString() != "via-var" {
		t.Errorf("EnvFunc = %q, want %q", got.AsString(), "via-var")
	}
}

func TestEnvNullArgErrors(t *testing.T) {
	if _, err := funcs.Env(nil).Call([]cty.Value{cty.NullVal(cty.String)}); err == nil {
		t.Error("env(null) error = nil, want non-nil (null and missing are different mistakes)")
	}
}

func TestNowIsUTC(t *testing.T) {
	before := time.Now().UTC().Add(-time.Minute)

	got, err := funcs.NowFunc.Call([]cty.Value{cty.StringVal(time.RFC3339)})
	if err != nil {
		t.Fatalf("now(RFC3339) error = %v, want nil", err)
	}

	parsed, err := time.Parse(time.RFC3339, got.AsString())
	if err != nil {
		t.Fatalf("time.Parse(now()) error = %v, want nil", err)
	}
	if zone, offset := parsed.Zone(); offset != 0 {
		t.Errorf("now(RFC3339) zone = %s (offset %d), want UTC", zone, offset)
	}
	if parsed.Before(before) || parsed.After(time.Now().UTC().Add(time.Minute)) {
		t.Errorf("now(RFC3339) = %v, want within a minute of current time", parsed)
	}
}

func TestNowNotMemoized(t *testing.T) {
	first, err := funcs.NowFunc.Call([]cty.Value{cty.StringVal(time.RFC3339Nano)})
	if err != nil {
		t.Fatalf("now() error = %v, want nil", err)
	}
	time.Sleep(5 * time.Millisecond)
	second, err := funcs.NowFunc.Call([]cty.Value{cty.StringVal(time.RFC3339Nano)})
	if err != nil {
		t.Fatalf("now() error = %v, want nil", err)
	}
	if first.AsString() == second.AsString() {
		t.Errorf("now() = %q twice, want distinct timestamps across invocations", first.AsString())
	}
}

func TestStdBundle(t *testing.T) {
	want := []string{"snakeCase", "camelCase", "pascalCase", "kebabCase", "now", "env"}

	std := funcs.Std()
	if len(std) != len(want) {
		t.Errorf("Std() len = %d, want %d", len(std), len(want))
	}
	for _, name := range want {
		if _, ok := std[name]; !ok {
			t.Errorf("Std()[%q] missing, want present", name)
		}
	}
}

func TestStdReturnsFreshMap(t *testing.T) {
	first := funcs.Std()
	delete(first, "env")

	if _, ok := funcs.Std()["env"]; !ok {
		t.Error(`Std()["env"] missing after mutating a prior copy, want fresh map per call`)
	}
}
