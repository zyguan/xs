package perms

// Rule -> Alt1 | Alt2 | ...
type Rule interface {
	Alts() []Alt
}

// Alt -> Elem1 Elem2 ...
type Alt interface {
	Elems() []Elem
}

// Elem -> Value | Rule
type Elem interface {
	IsRule() bool
	Rule() Rule
	Value() interface{}
}

type some struct{ x interface{} }

func (v some) IsRule() bool { return false }

func (v some) Rule() Rule { return nil }

func (v some) Value() interface{} { return v.x }

type alts []Alt

func (r alts) Alts() []Alt { return r }

type ruleElem struct{ r Rule }

func (re ruleElem) IsRule() bool { return true }

func (re ruleElem) Rule() Rule { return re.r }

func (re ruleElem) Value() interface{} { return nil }

type alt []Elem

func (a alt) Elems() []Elem { return a }

func R(as ...Alt) Rule     { return alts(as) }
func A(elems ...Elem) Alt  { return alt(elems) }
func V(x interface{}) Elem { return some{x} }
func E(r Rule) Elem        { return ruleElem{r} }

func Seq(xs ...interface{}) Rule {
	elems := make([]Elem, len(xs))
	for i, x := range xs {
		switch e := x.(type) {
		case Elem:
			elems[i] = e
		case Alt:
			elems[i] = E(R(e))
		case Rule:
			elems[i] = E(e)
		default:
			elems[i] = V(x)
		}
	}
	return R(A(elems...))
}

func OneOf(xs ...interface{}) Rule {
	alts := make([]Alt, len(xs))
	for i, x := range xs {
		switch a := x.(type) {
		case Alt:
			alts[i] = a
		case Rule:
			if len(a.Alts()) == 1 {
				alts[i] = a.Alts()[0]
			} else {
				alts[i] = A(E(a))
			}
		case Elem:
			alts[i] = A(a)
		default:
			alts[i] = A(V(a))
		}
	}
	return R(alts...)
}

func Empty() Alt {
	return A()
}

func Walk(root Rule, cb func(...interface{})) {
	type end int

	state := make([]interface{}, 0, 64)
	remaining := []interface{}{end(0), root}

	pop := func() interface{} {
		size := len(remaining)
		if size == 0 {
			return nil
		}
		last := remaining[size-1]
		remaining = remaining[:size-1]
		return last
	}
	lastRegion := func() (int, int) {
		for i := len(remaining) - 1; i >= 0; i-- {
			if _, ok := remaining[i].(end); ok {
				return i, len(remaining)
			}
		}
		return -1, -1
	}

	for len(remaining) != 0 {
		switch v := pop().(type) {
		case Rule:
			i, j := lastRegion()
			alts := v.Alts()
			size := len(alts)
			if size == 0 {
				continue
			}
			remaining = append(remaining, alts[size-1])
			for k := size - 2; k >= 0; k-- {
				remaining = append(remaining, end(len(state)))
				remaining = append(remaining, remaining[i+1:j]...)
				remaining = append(remaining, alts[k])
			}
		case Alt:
			elems := v.Elems()
			size := len(elems)
			for k := size - 1; k >= 0; k-- {
				remaining = append(remaining, elems[k])
			}
		case Elem:
			if v.IsRule() {
				remaining = append(remaining, v.Rule())
			} else {
				state = append(state, v.Value())
			}
		case end:
			cb(state...)
			state = state[:int(v)]
		}
	}
}
