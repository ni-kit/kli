package domain

type ToggleState int

const (
	ToggleStateZero ToggleState = iota
	ToggleStateOne
)

type Toggle struct {
	Cwd   string      `json:"cwd"`
	Zero  *Invocation `json:"zero,omitempty"`
	One   *Invocation `json:"one,omitempty"`
	State ToggleState `json:"state"`
}

func (t Toggle) Current() *Invocation {
	if t.State == ToggleStateOne {
		return t.One
	}
	return t.Zero
}

func (t Toggle) Alternative() *Invocation {
	if t.State == ToggleStateOne {
		return t.Zero
	}
	return t.One
}

func (t Toggle) AlternativeState() ToggleState {
	if t.State == ToggleStateOne {
		return ToggleStateZero
	}
	return ToggleStateOne
}
