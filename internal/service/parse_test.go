package service

import (
	"testing"

	"github.com/ni-kit/kli/internal/domain"
)

func TestExtractRedirects(t *testing.T) {
	tests := []struct {
		name          string
		tokens        []string
		wantRemaining []string
		wantStdout    domain.StreamRedirect
		wantStderr    domain.StreamRedirect
	}{
		{
			name:          "2>&1 and append stdout",
			tokens:        []string{"test", "2>&1", ">>filename.txt"},
			wantRemaining: []string{"test"},
			wantStdout:    domain.StreamRedirect{Target: domain.RedirectFile, File: "filename.txt", Append: true},
			wantStderr:    domain.StreamRedirect{Target: domain.RedirectToOther},
		},
		{
			name:          "stdout to file overwrite",
			tokens:        []string{"arg", ">out.txt"},
			wantRemaining: []string{"arg"},
			wantStdout:    domain.StreamRedirect{Target: domain.RedirectFile, File: "out.txt"},
		},
		{
			name:          "stderr to file two-token form",
			tokens:        []string{"2>", "err.log"},
			wantRemaining: nil,
			wantStderr:    domain.StreamRedirect{Target: domain.RedirectFile, File: "err.log"},
		},
		{
			name:          "stdout to null",
			tokens:        []string{">/dev/null"},
			wantRemaining: nil,
			wantStdout:    domain.StreamRedirect{Target: domain.RedirectNull},
		},
		{
			name:          "stdout to stderr",
			tokens:        []string{">&2"},
			wantRemaining: nil,
			wantStdout:    domain.StreamRedirect{Target: domain.RedirectToOther},
		},
		{
			name:          "no redirects",
			tokens:        []string{"--flag", "value"},
			wantRemaining: []string{"--flag", "value"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			remaining, stdout, stderr := extractRedirects(tc.tokens)
			if len(remaining) != len(tc.wantRemaining) {
				t.Fatalf("remaining: got %v, want %v", remaining, tc.wantRemaining)
			}
			for i, r := range remaining {
				if r != tc.wantRemaining[i] {
					t.Errorf("remaining[%d]: got %q, want %q", i, r, tc.wantRemaining[i])
				}
			}
			if stdout != tc.wantStdout {
				t.Errorf("stdout: got %+v, want %+v", stdout, tc.wantStdout)
			}
			if stderr != tc.wantStderr {
				t.Errorf("stderr: got %+v, want %+v", stderr, tc.wantStderr)
			}
		})
	}
}

func TestParseChain(t *testing.T) {
	tests := []struct {
		name      string
		argv      []string
		wantChain []struct {
			op      domain.ChainOp
			command string
		}
	}{
		{
			name: "and chain",
			argv: []string{"git", "add", ".", "&&", "git", "commit", "-m", "msg"},
			wantChain: []struct {
				op      domain.ChainOp
				command string
			}{
				{domain.ChainAnd, "git"},
			},
		},
		{
			name: "pipe chain",
			argv: []string{"ls", "-la", "|", "grep", "foo"},
			wantChain: []struct {
				op      domain.ChainOp
				command string
			}{
				{domain.ChainPipe, "grep"},
			},
		},
		{
			name: "three commands",
			argv: []string{"a", "&&", "b", "|", "c"},
			wantChain: []struct {
				op      domain.ChainOp
				command string
			}{
				{domain.ChainAnd, "b"},
				{domain.ChainPipe, "c"},
			},
		},
		{
			name:      "no chain",
			argv:      []string{"git", "status"},
			wantChain: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			inv := Parse(tc.argv)
			if len(inv.Chain) != len(tc.wantChain) {
				t.Fatalf("chain len: got %d, want %d", len(inv.Chain), len(tc.wantChain))
			}
			for i, w := range tc.wantChain {
				got := inv.Chain[i]
				if got.Op != w.op {
					t.Errorf("chain[%d].Op: got %q, want %q", i, got.Op, w.op)
				}
				if got.Command != w.command {
					t.Errorf("chain[%d].Command: got %q, want %q", i, got.Command, w.command)
				}
			}
		})
	}
}

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
