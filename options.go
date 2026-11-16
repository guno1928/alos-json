package json

// Options controls the behavior of Marshal and Unmarshal.
type Options struct {
	flags        uint64
	set          uint64
	indent       string
	prefix       string
	marshalers   *Marshalers
	unmarshalers *Unmarshalers
}

const (
	flagOmitZeroStructFields uint64 = 1 << iota
	flagMatchCaseInsensitive
	flagStringifyNumbers
	flagRejectUnknownMembers
	flagDeterministic
	flagFormatNilSliceAsNull
	flagFormatNilMapAsNull
	flagEscapeHTML
	flagMultiline
	flagOmitEmpty
)

// Option is a functional option for Marshal and Unmarshal.
type Option func(*Options)

// OmitZeroStructFields configures whether zero-valued struct fields are omitted.
func OmitZeroStructFields(v bool) Option {
	return func(o *Options) {
		o.set |= flagOmitZeroStructFields
		if v {
			o.flags |= flagOmitZeroStructFields
		} else {
			o.flags &^= flagOmitZeroStructFields
		}
	}
}

// MatchCaseInsensitiveNames configures case-insensitive field name matching.
func MatchCaseInsensitiveNames(v bool) Option {
	return func(o *Options) {
		o.set |= flagMatchCaseInsensitive
		if v {
			o.flags |= flagMatchCaseInsensitive
		} else {
			o.flags &^= flagMatchCaseInsensitive
		}
	}
}

// StringifyNumbers configures whether numbers are encoded as strings.
func StringifyNumbers(v bool) Option {
	return func(o *Options) {
		o.set |= flagStringifyNumbers
		if v {
			o.flags |= flagStringifyNumbers
		} else {
			o.flags &^= flagStringifyNumbers
		}
	}
}

// RejectUnknownMembers configures whether unknown struct fields cause an error.
func RejectUnknownMembers(v bool) Option {
	return func(o *Options) {
		o.set |= flagRejectUnknownMembers
		if v {
			o.flags |= flagRejectUnknownMembers
		} else {
			o.flags &^= flagRejectUnknownMembers
		}
	}
}

// Deterministic configures whether map keys are sorted for deterministic output.
func Deterministic(v bool) Option {
	return func(o *Options) {
		o.set |= flagDeterministic
		if v {
			o.flags |= flagDeterministic
		} else {
			o.flags &^= flagDeterministic
		}
	}
}

// FormatNilSliceAsNull configures whether nil slices are encoded as null instead of [].
func FormatNilSliceAsNull(v bool) Option {
	return func(o *Options) {
		o.set |= flagFormatNilSliceAsNull
		if v {
			o.flags |= flagFormatNilSliceAsNull
		} else {
			o.flags &^= flagFormatNilSliceAsNull
		}
	}
}

// FormatNilMapAsNull configures whether nil maps are encoded as null instead of {}.
func FormatNilMapAsNull(v bool) Option {
	return func(o *Options) {
		o.set |= flagFormatNilMapAsNull
		if v {
			o.flags |= flagFormatNilMapAsNull
		} else {
			o.flags &^= flagFormatNilMapAsNull
		}
	}
}

// EscapeHTML configures whether HTML characters are escaped in JSON strings.
func EscapeHTML(v bool) Option {
	return func(o *Options) {
		o.set |= flagEscapeHTML
		if v {
			o.flags |= flagEscapeHTML
		} else {
			o.flags &^= flagEscapeHTML
		}
	}
}

// WithIndent configures pretty-printing with the given prefix and indent.
func WithIndent(prefix, indent string) Option {
	return func(o *Options) {
		o.prefix = prefix
		o.indent = indent
		o.set |= flagMultiline
		if indent != "" || prefix != "" {
			o.flags |= flagMultiline
		} else {
			o.flags &^= flagMultiline
		}
	}
}

// WithMarshalers sets custom type-specific marshalers.
func WithMarshalers(m *Marshalers) Option {
	return func(o *Options) {
		o.marshalers = m
	}
}

// WithUnmarshalers sets custom type-specific unmarshalers.
func WithUnmarshalers(u *Unmarshalers) Option {
	return func(o *Options) {
		o.unmarshalers = u
	}
}

// DefaultOptionsV2 returns an option that resets all flags to v2 defaults.
func DefaultOptionsV2() Option {
	return func(o *Options) {
		o.flags = 0
		o.set = 0xFFFFFFFFFFFFFFFF
	}
}

func (o *Options) has(flag uint64) bool {
	return o.flags&flag != 0
}

func buildOptions(opts []Option) Options {
	var o Options
	for _, fn := range opts {
		fn(&o)
	}
	return o
}
