package jsontext

type stateEntry uint64

const (
	stateTypeMask     stateEntry = 1 << 63
	stateTypeObject   stateEntry = 1 << 63
	stateTypeArray    stateEntry = 0
	stateCountMask    stateEntry = ^stateTypeMask
	stateCountLSBMask stateEntry = 1
)

func (e stateEntry) isObject() bool        { return e&stateTypeMask != 0 }
func (e stateEntry) isArray() bool         { return e&stateTypeMask == 0 }
func (e stateEntry) length() uint64        { return uint64(e & stateCountMask) }
func (e stateEntry) needObjectName() bool  { return e&stateTypeMask != 0 && e&stateCountLSBMask == 0 }
func (e stateEntry) needObjectValue() bool { return e&stateTypeMask != 0 && e&stateCountLSBMask != 0 }
func (e *stateEntry) increment()           { *e++ }

type stateMachine struct {
	stack []stateEntry
	last  stateEntry
	depth int
}

func (sm *stateMachine) init() {
	sm.stack = sm.stack[:0]
	sm.last = stateTypeArray
	sm.depth = 0
}

func (sm *stateMachine) Depth() int { return sm.depth }

func (sm *stateMachine) pushObject() {
	sm.stack = append(sm.stack, sm.last)
	sm.last = stateTypeObject
	sm.depth++
}

func (sm *stateMachine) pushArray() {
	sm.stack = append(sm.stack, sm.last)
	sm.last = stateTypeArray
	sm.depth++
}

func (sm *stateMachine) popObject() bool {
	if sm.depth == 0 || !sm.last.isObject() {
		return false
	}
	if sm.last.needObjectValue() {
		return false
	}
	sm.depth--
	sm.last = sm.stack[sm.depth]
	sm.stack = sm.stack[:sm.depth]
	sm.last.increment()
	return true
}

func (sm *stateMachine) popArray() bool {
	if sm.depth == 0 || !sm.last.isArray() {
		return false
	}
	sm.depth--
	sm.last = sm.stack[sm.depth]
	sm.stack = sm.stack[:sm.depth]
	sm.last.increment()
	return true
}

func (sm *stateMachine) appendValue() bool {
	if sm.last.needObjectName() {
		return false
	}
	sm.last.increment()
	return true
}

func (sm *stateMachine) appendName() bool {
	if !sm.last.needObjectName() {
		return false
	}
	sm.last.increment()
	return true
}

func (sm *stateMachine) needDelim(next Kind) byte {
	if sm.depth == 0 {
		return 0
	}
	e := sm.last
	switch {
	case e.needObjectValue():
		return ':'
	case e.length() > 0 && next != EndObject && next != EndArray:
		return ','
	default:
		return 0
	}
}
