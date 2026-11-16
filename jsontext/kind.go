package jsontext

type Kind byte

const (
	Invalid     Kind = 0
	Null        Kind = 'n'
	False       Kind = 'f'
	True        Kind = 't'
	String      Kind = '"'
	Number      Kind = '0'
	BeginObject Kind = '{'
	EndObject   Kind = '}'
	BeginArray  Kind = '['
	EndArray    Kind = ']'
)

func (k Kind) String() string {
	switch k {
	case Null:
		return "null"
	case False:
		return "false"
	case True:
		return "true"
	case String:
		return "string"
	case Number:
		return "number"
	case BeginObject:
		return "{"
	case EndObject:
		return "}"
	case BeginArray:
		return "["
	case EndArray:
		return "]"
	default:
		return "invalid"
	}
}

var kindTable [256]Kind

func init() {
	kindTable['n'] = Null
	kindTable['f'] = False
	kindTable['t'] = True
	kindTable['"'] = String
	kindTable['{'] = BeginObject
	kindTable['}'] = EndObject
	kindTable['['] = BeginArray
	kindTable[']'] = EndArray
	kindTable['-'] = Number
	for c := byte('0'); c <= '9'; c++ {
		kindTable[c] = Number
	}
}

func normalizeKind(c byte) Kind {
	return kindTable[c]
}
