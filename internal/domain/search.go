package domain

import (
	"slices"
	"strings"
)

type SearchQuery struct {
	Commands []string // C: prefix
	Flags    []string // F: prefix
	Values   []string // V: prefix
	Paths    []string // P: prefix
	Tags     []string // T: prefix
	Date     string   // D: prefix (substring match on formatted date)
	RunsAsc  *bool    // R:asc → true, R:desc → false, absent → nil
	FreeText string   // everything left after prefixed tokens
}

func ParseQuery(input string) SearchQuery {
	tokens := SplitShellLine(input)
	var q SearchQuery
	var free []string

	for _, tok := range tokens {
		switch {
		case hasPrefix(tok, "C:"):
			q.Commands = append(q.Commands, tok[2:])
		case hasPrefix(tok, "F:"):
			q.Flags = append(q.Flags, tok[2:])
		case hasPrefix(tok, "V:"):
			q.Values = append(q.Values, tok[2:])
		case hasPrefix(tok, "P:"):
			q.Paths = append(q.Paths, tok[2:])
		case hasPrefix(tok, "T:"):
			q.Tags = append(q.Tags, tok[2:])
		case hasPrefix(tok, "D:"):
			q.Date = tok[2:]
		case strings.EqualFold(tok, "R:asc"):
			t := true
			q.RunsAsc = &t
		case strings.EqualFold(tok, "R:desc"):
			f := false
			q.RunsAsc = &f
		default:
			free = append(free, tok)
		}
	}
	q.FreeText = strings.Join(free, " ")
	return q
}

func hasPrefix(s, prefix string) bool {
	return len(s) > len(prefix) && strings.EqualFold(s[:len(prefix)], prefix)
}

func (q SearchQuery) IsEmpty() bool {
	return len(q.Commands) == 0 && len(q.Flags) == 0 && len(q.Values) == 0 &&
		len(q.Paths) == 0 && len(q.Tags) == 0 && q.Date == "" &&
		q.RunsAsc == nil && q.FreeText == ""
}

func (q SearchQuery) Match(inv Invocation) bool {
	for _, c := range q.Commands {
		if !containsFold(inv.Command, c) {
			return false
		}
	}

	for _, f := range q.Flags {
		var matched bool
		for _, a := range inv.Args {
			if (a.Kind == ArgShortFlag || a.Kind == ArgLongFlag) && containsFold(a.Name, f) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	for _, v := range q.Values {
		var matched bool
		for _, a := range inv.Args {
			if containsFold(a.Value, v) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	for _, p := range q.Paths {
		var matched bool
		for _, r := range inv.Runs {
			if containsFold(r.Cwd, p) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	for _, t := range q.Tags {
		var matched bool
		for _, tag := range inv.Tags {
			if containsFold(tag, t) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	if q.Date != "" {
		last := inv.LastRun()
		if !containsFold(last.RunAt.Format("2006-01-02"), q.Date) &&
			!containsFold(last.RunAt.Format("02/01/2006"), q.Date) {
			return false
		}
	}

	if q.FreeText != "" {
		if !containsFold(inv.FullCommand(), q.FreeText) {
			return false
		}
	}

	return true
}

func (q SearchQuery) Filter(invs []Invocation) []Invocation {
	if q.IsEmpty() {
		return invs
	}
	out := make([]Invocation, 0, len(invs))
	for _, inv := range invs {
		if q.Match(inv) {
			out = append(out, inv)
		}
	}
	return out
}

func (q SearchQuery) Sort(invs []Invocation) {
	if q.RunsAsc == nil {
		return
	}
	asc := *q.RunsAsc
	slices.SortStableFunc(invs, func(a, b Invocation) int {
		la, lb := len(a.Runs), len(b.Runs)
		if asc {
			return la - lb
		}
		return lb - la
	})
}

func containsFold(s, sub string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(sub))
}

func SplitShellLine(line string) []string {
	var (
		tokens   []string
		cur      strings.Builder
		inSingle bool
		inDouble bool
	)

	for i := 0; i < len(line); i++ {
		ch := line[i]
		switch {
		case inSingle:
			if ch == '\'' {
				inSingle = false
			} else {
				cur.WriteByte(ch)
			}
		case inDouble:
			if ch == '"' {
				inDouble = false
			} else {
				cur.WriteByte(ch)
			}
		case ch == '\'':
			inSingle = true
		case ch == '"':
			inDouble = true
		case ch == ' ' || ch == '\t':
			if cur.Len() > 0 {
				tokens = append(tokens, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteByte(ch)
		}
	}
	if cur.Len() > 0 {
		tokens = append(tokens, cur.String())
	}
	return tokens
}
