package domain

type ToggleState int

const (
	ToggleStateZero ToggleState = iota
	ToggleStateOne
)

type Toggle struct {
	Cwd          string       `json:"cwd"`
	Zero         *Invocation  `json:"zero,omitempty"`
	One          *Invocation  `json:"one,omitempty"`
	State        ToggleState  `json:"state"`
	NextAddState *ToggleState `json:"next_add_state,omitempty"`
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

func (t Toggle) NextAddSlot() ToggleState {
	if t.One == nil {
		return ToggleStateOne
	}
	if t.Zero == nil {
		return ToggleStateZero
	}
	if t.NextAddState != nil {
		return *t.NextAddState
	}
	return ToggleStateOne
}

func NextToggleState(state ToggleState) ToggleState {
	if state == ToggleStateOne {
		return ToggleStateZero
	}
	return ToggleStateOne
}
