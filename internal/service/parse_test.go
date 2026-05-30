package service

import (
	"testing"

	"github.com/ni-kit/kli/internal/domain"
)

func TestParse(t *testing.T) {
	type wantArg struct {
		kind  domain.ArgKind
		name  string
		value string
	}
	type wantEnv struct {
		key   string
		value string
	}
	tests := []struct {
		name    string
		argv    []string
		command string
		env     []wantEnv
		args    []wantArg
	}{
		{
			name:    "leading env vars",
			argv:    []string{"TELEGRAM_TOKEN=some", "JWT_SECRET=another", "go", "test", "./..."},
			command: "go",
			env: []wantEnv{
				{"TELEGRAM_TOKEN", "some"},
				{"JWT_SECRET", "another"},
			},
			args: []wantArg{
				{domain.ArgPositional, "", "test"},
				{domain.ArgPositional, "", "./..."},
			},
		},
		{
			name:    "short flag cluster",
			argv:    []string{"git", "commit", "-am", "initial commit"},
			command: "git",
			args: []wantArg{
				{domain.ArgPositional, "", "commit"},
				{domain.ArgShortFlag, "a", ""},
				{domain.ArgShortFlag, "m", ""},
				{domain.ArgFlagValue, "", "initial commit"},
			},
		},
		{
			name:    "short flag with value",
			argv:    []string{"kubectl", "get", "pods", "-n", "kube-system"},
			command: "kubectl",
			args: []wantArg{
				{domain.ArgPositional, "", "get"},
				{domain.ArgPositional, "", "pods"},
				{domain.ArgShortFlag, "n", ""},
				{domain.ArgFlagValue, "", "kube-system"},
			},
		},
		{
			name:    "long flag with separate value",
			argv:    []string{"git", "push", "--set-upstream", "origin", "main"},
			command: "git",
			args: []wantArg{
				{domain.ArgPositional, "", "push"},
				{domain.ArgLongFlag, "set-upstream", ""},
				{domain.ArgFlagValue, "", "origin"},
				{domain.ArgPositional, "", "main"},
			},
		},
		{
			name:    "long flag equals syntax",
			argv:    []string{"go", "test", "--run=TestFoo"},
			command: "go",
			args: []wantArg{
				{domain.ArgPositional, "", "test"},
				{domain.ArgLongFlag, "run", "TestFoo"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			inv := Parse(tc.argv)
			if inv.Command != tc.command {
				t.Fatalf("command: got %q, want %q", inv.Command, tc.command)
			}
			if len(inv.Env) != len(tc.env) {
				t.Fatalf("env len: got %d, want %d — env: %+v", len(inv.Env), len(tc.env), inv.Env)
			}
			for i, w := range tc.env {
				got := inv.Env[i]
				if got.Key != w.key || got.Value != w.value {
					t.Errorf("env[%d]: got {%q %q}, want {%q %q}", i, got.Key, got.Value, w.key, w.value)
				}
			}
			if len(inv.Args) != len(tc.args) {
				t.Fatalf("args len: got %d, want %d — args: %+v", len(inv.Args), len(tc.args), inv.Args)
			}
			for i, w := range tc.args {
				got := inv.Args[i]
				if got.Kind != w.kind || got.Name != w.name || got.Value != w.value {
					t.Errorf("arg[%d]: got {%v %q %q}, want {%v %q %q}", i, got.Kind, got.Name, got.Value, w.kind, w.name, w.value)
				}
			}
		})
	}
}
