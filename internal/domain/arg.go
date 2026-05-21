package domain

type ArgKind int

const (
	ArgPositional ArgKind = iota
	ArgShortFlag
	ArgLongFlag
	ArgFlagValue
)

func (k ArgKind) String() string {
	switch k {
	case ArgPositional:
		return "positional"
	case ArgShortFlag:
		return "short flag"
	case ArgLongFlag:
		return "long flag"
	case ArgFlagValue:
		return "value"
	}
	return "unknown"
}

type Arg struct {
	Kind     ArgKind
	Name     string
	Value    string
	Redacted bool
}

func (a Arg) Display() string {
	switch a.Kind {
	case ArgShortFlag:
		return "-" + a.Name
	case ArgLongFlag:
		if a.Value != "" {
			return "--" + a.Name + "=" + a.Value
		}
		return "--" + a.Name
	case ArgFlagValue, ArgPositional:
		if a.Redacted {
			return "••••"
		}
		return a.Value
	}
	return ""
}

func (a Arg) RawDisplay() string {
	switch a.Kind {
	case ArgShortFlag:
		return "-" + a.Name
	case ArgLongFlag:
		if a.Value != "" {
			return "--" + a.Name + "=" + a.Value
		}
		return "--" + a.Name
	case ArgFlagValue, ArgPositional:
		return a.Value
	}
	return ""
}
