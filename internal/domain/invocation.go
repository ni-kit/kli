package domain

import (
	"slices"
	"strings"
	"time"
)

type Run struct {
	RunAt time.Time
	Cwd   string
}

type Invocation struct {
	ID      string
	Command string
	Args    []Arg
	Runs    []Run // newest-first
	Tags    []string
}

func (inv Invocation) LastRun() Run {
	if len(inv.Runs) == 0 {
		return Run{}
	}
	return inv.Runs[0]
}

func (inv *Invocation) AddRun(r Run) {
	inv.Runs = append([]Run{r}, inv.Runs...)
}

func (inv Invocation) ArgsString() string {
	parts := make([]string, 0, len(inv.Args))
	for _, a := range inv.Args {
		parts = append(parts, a.Display())
	}
	return strings.Join(parts, " ")
}

func (inv Invocation) ArgsPreview() string {
	s := inv.ArgsString()
	if len(s) > 50 {
		s = s[:47] + "…"
	}
	return s
}

func (inv Invocation) FullCommand() string {
	return inv.Command + " " + inv.ArgsString()
}

func (inv Invocation) CommandFingerprint() string {
	var flags []string
	var positionals []string
	for _, a := range inv.Args {
		switch a.Kind {
		case ArgShortFlag, ArgLongFlag:
			flags = append(flags, a.Name+"="+a.Value)
		case ArgPositional, ArgFlagValue:
			positionals = append(positionals, a.Value)
		}
	}
	slices.Sort(flags)
	return inv.Command + "|" + strings.Join(flags, ",") + "|" + strings.Join(positionals, ",")
}

func (inv Invocation) FlagSetFingerprint() string {
	var pairs []string
	for _, a := range inv.Args {
		switch a.Kind {
		case ArgShortFlag, ArgLongFlag:
			pairs = append(pairs, a.Name+"="+a.Value)
		}
	}
	slices.Sort(pairs)
	return strings.Join(pairs, ",")
}

func (inv Invocation) HasTag(tag string) bool {
	return slices.Contains(inv.Tags, tag)
}
