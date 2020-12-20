package rule

import "errors"

var ErrWrongType = errors.New("wrong type of element")

// Rule -> Alt1 | Alt2 | ...
type Rule interface {
	Alts() []Alt
}

// Alt -> Elem1 Elem2 ...
type Alt interface {
	Elems() []Elem
}

// Elem -> Str | Rule
type Elem interface {
	IsRule() bool
	Rule() Rule
	Value() string
}

type S string

func (s S) IsRule() bool { return false }

func (s S) Rule() Rule { return nil }

func (s S) Value() string { return string(s) }

type rule []Alt

func (r rule) Alts() []Alt { return r }

func R(alts ...Alt) Rule { return rule(alts) }

type ruleElem struct{ r Rule }

func (re ruleElem) IsRule() bool { return true }

func (re ruleElem) Rule() Rule { return re.r }

func (re ruleElem) Value() string { return "" }

func E(r Rule) Elem { return ruleElem{r} }

type alt []Elem

func (a alt) Elems() []Elem { return a }

func A(elems ...Elem) Alt { return alt(elems) }

func Seq(xs ...interface{}) Rule {
	elems := make([]Elem, len(xs))
	for i, x := range xs {
		switch e := x.(type) {
		case Elem:
			elems[i] = e
		case string:
			elems[i] = S(e)
		case Alt:
			elems[i] = E(R(e))
		case Rule:
			elems[i] = E(e)
		default:
			panic(ErrWrongType)
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
		case string:
			alts[i] = A(S(a))
		default:
			panic(ErrWrongType)
		}
	}
	return R(alts...)
}

func Empty() Alt {
	return A()
}

func Walk(root Rule, cb func([]string)) {
	type end int

	state := make([]string, 0, 64)
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
			ss := make([]string, len(state))
			copy(ss, state)
			cb(ss)
			state = state[:int(v)]
		}
	}
}
