package service

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ni-kit/kli/internal/domain"
)

func Parse(argv []string) domain.Invocation {
	cwd, _ := os.Getwd()
	return parseWithTime(argv, time.Now(), cwd)
}

func parseWithTime(argv []string, t time.Time, cwd string) domain.Invocation {
	inv := domain.Invocation{
		ID:      newID(),
		Command: argv[0],
		Runs:    []domain.Run{{RunAt: t, Cwd: cwd}},
	}

	tokens := argv[1:]
	for i := 0; i < len(tokens); i++ {
		tok := tokens[i]
		switch {
		case strings.HasPrefix(tok, "--"):
			name := tok[2:]
			if idx := strings.IndexByte(name, '='); idx >= 0 {
				inv.Args = append(inv.Args, domain.Arg{Kind: domain.ArgLongFlag, Name: name[:idx], Value: name[idx+1:]})
			} else if i+1 < len(tokens) && !strings.HasPrefix(tokens[i+1], "-") {
				inv.Args = append(inv.Args, domain.Arg{Kind: domain.ArgLongFlag, Name: name})
				i++
				inv.Args = append(inv.Args, domain.Arg{Kind: domain.ArgFlagValue, Value: tokens[i]})
			} else {
				inv.Args = append(inv.Args, domain.Arg{Kind: domain.ArgLongFlag, Name: name})
			}

		case strings.HasPrefix(tok, "-") && len(tok) > 1:
			letters := tok[1:]
			for j, ch := range letters {
				inv.Args = append(inv.Args, domain.Arg{Kind: domain.ArgShortFlag, Name: string(ch)})
				if j == len(letters)-1 && i+1 < len(tokens) && !strings.HasPrefix(tokens[i+1], "-") {
					i++
					inv.Args = append(inv.Args, domain.Arg{Kind: domain.ArgFlagValue, Value: tokens[i]})
				}
			}

		default:
			inv.Args = append(inv.Args, domain.Arg{Kind: domain.ArgPositional, Value: tok})
		}
	}
	return inv
}

func newID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
