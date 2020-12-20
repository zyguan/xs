package gen

import (
	"context"
	"errors"
	"math/rand"
	"time"
)

var (
	Pending       = errors.New("pending")
	StopIteration = errors.New("stop iteration")
)

func IsPending(x interface{}) bool {
	if e, ok := x.(error); ok {
		return errors.Is(e, Pending)
	}
	return false
}

func IsStopIteration(x interface{}) bool {
	if e, ok := x.(error); ok {
		return errors.Is(e, StopIteration)
	}
	return false
}

type Generator interface {
	Update(ctx context.Context) Generator
	Next(ctx context.Context) (interface{}, Generator)
}

func WrapAllNonNil(xs []interface{}) []Generator {
	if len(xs) == 0 {
		return nil
	}
	gs := make([]Generator, 0, len(xs))
	for _, x := range xs {
		if x == nil {
			continue
		}
		gs = append(gs, Some(x))
	}
	if len(gs) == 0 {
		return nil
	}
	return gs
}

func UpdateAll(ctx context.Context, gs []Generator) []Generator {
	out := make([]Generator, 0, len(gs))
	for _, g := range gs {
		if g == nil {
			continue
		}
		if ug := g.Update(ctx); ug != nil {
			out = append(out, ug)
		}
	}
	return out
}

func None() Generator { return nil }

func Some(val interface{}) Generator {
	switch g := val.(type) {
	case Generator:
		return g
	case func() interface{}:
		return fn0(g)
	case func(context.Context) interface{}:
		return fn1(g)
	default:
		return some{val}
	}
}

type some struct{ val interface{} }

func (g some) Update(ctx context.Context) Generator { return g }

func (g some) Next(ctx context.Context) (interface{}, Generator) { return g.val, nil }

type fn0 func() interface{}

func (g fn0) Update(ctx context.Context) Generator { return g }

func (g fn0) Next(ctx context.Context) (interface{}, Generator) {
	if g == nil {
		return StopIteration, nil
	}
	return Cons(Some(g()), g).Next(ctx)
}

type fn1 func(ctx context.Context) interface{}

func (g fn1) Update(ctx context.Context) Generator { return g }

func (g fn1) Next(ctx context.Context) (interface{}, Generator) {
	if g == nil {
		return StopIteration, nil
	}
	return Cons(Some(g(ctx)), g).Next(ctx)
}

func Cons(head Generator, tail Generator) Generator {
	if head == nil {
		return tail
	}
	if tail == nil {
		return head
	}
	return cons{head, tail}
}

type cons struct {
	head Generator
	tail Generator
}

func (g cons) Update(ctx context.Context) Generator {
	if g.head != nil {
		g.head = g.head.Update(ctx)
	}
	if g.tail != nil {
		g.tail = g.tail.Update(ctx)
	}
	return g
}

func (g cons) Next(ctx context.Context) (interface{}, Generator) {
	if g.head == nil {
		if g.tail == nil {
			return StopIteration, nil
		}
		return g.tail.Next(ctx)
	}
	x, ng := g.head.Next(ctx)
	if ng == nil {
		ng = g.tail
	} else {
		ng = cons{ng, g.tail}
	}
	return x, ng
}

func Seq(xs ...interface{}) Generator {
	gs := WrapAllNonNil(xs)
	if len(gs) == 0 {
		return nil
	}
	return seq(gs)
}

type seq []Generator

func (gs seq) Update(ctx context.Context) Generator {
	ng := UpdateAll(ctx, gs)
	if len(ng) == 0 {
		return nil
	}
	return seq(ng)
}

func (gs seq) Next(ctx context.Context) (interface{}, Generator) {
	for i, g := range gs {
		if g == nil {
			continue
		}
		tail := gs[i+1:]
		if len(tail) == 0 {
			return g.Next(ctx)
		}
		return Cons(g, tail).Next(ctx)
	}
	return StopIteration, nil
}

func Mix(xs ...interface{}) Generator {
	gs := WrapAllNonNil(xs)
	if len(gs) == 0 {
		return nil
	}
	return mix(gs)
}

type mix []Generator

func (gs mix) Update(ctx context.Context) Generator {
	ng := UpdateAll(ctx, gs)
	if len(ng) == 0 {
		return nil
	}
	return mix(ng)
}

func (gs mix) Next(ctx context.Context) (interface{}, Generator) {
	alts := make([]Generator, 0, len(gs))
	for _, g := range gs {
		if g == nil {
			continue
		}
		alts = append(alts, g)
	}
	if len(alts) == 0 {
		return StopIteration, nil
	}
	i := rand.Intn(len(alts))
	x, ng := alts[i].Next(ctx)
	if len(alts) == 1 && ng == nil {
		return x, nil
	}
	alts[i] = ng
	return x, mix(alts)
}

func Map(f func(x interface{}) interface{}, g Generator) Generator {
	if g == nil {
		return nil
	}
	return mapper{g, f}
}

type mapper struct {
	inner Generator
	f     func(interface{}) interface{}
}

func (g mapper) Update(ctx context.Context) Generator {
	if g.inner == nil {
		return nil
	}
	return Map(g.f, g.inner.Update(ctx))
}

func (g mapper) Next(ctx context.Context) (interface{}, Generator) {
	if g.inner == nil {
		return StopIteration, nil
	}
	x, ng := g.inner.Next(ctx)
	return g.f(x), Map(g.f, ng)
}

func FlatMap(f func(x interface{}) Generator, g Generator) Generator {
	if g == nil {
		return nil
	}
	return flatMapper{g, f}
}

func Filter(f func(x interface{}) bool, g Generator) Generator {
	return FlatMap(func(x interface{}) Generator {
		if f(x) {
			return Some(x)
		} else {
			return nil
		}
	}, g)
}

type flatMapper struct {
	inner Generator
	f     func(interface{}) Generator
}

func (g flatMapper) Update(ctx context.Context) Generator {
	if g.inner == nil {
		return nil
	}
	return FlatMap(g.f, g.inner.Update(ctx))
}

func (g flatMapper) Next(ctx context.Context) (interface{}, Generator) {
	if g.inner == nil {
		return StopIteration, nil
	}
	x, ng := g.inner.Next(ctx)
	if ng != nil {
		ng = FlatMap(g.f, ng)
	}
	return Cons(g.f(x), ng).Next(ctx)
}

func Once(g Generator) Generator { return Limit(1, g) }

func Limit(n int, g Generator) Generator {
	if g == nil || n <= 0 {
		return nil
	}
	return limit{g, n}
}

type limit struct {
	inner     Generator
	remaining int
}

func (g limit) Update(ctx context.Context) Generator {
	if g.inner == nil {
		return nil
	}
	return Limit(g.remaining, g.inner.Update(ctx))
}

func (g limit) Next(ctx context.Context) (interface{}, Generator) {
	if g.remaining <= 0 || g.inner == nil {
		return StopIteration, nil
	}
	x, ng := g.inner.Next(ctx)
	return x, Limit(g.remaining-1, ng)
}

func Repeat(g Generator) Generator {
	if g == nil {
		return nil
	}
	return repeat{g, g}
}

type repeat struct {
	orig Generator
	iter Generator
}

func (g repeat) Update(ctx context.Context) Generator {
	if g.orig == nil {
		return nil
	}
	if g.iter == nil {
		g.iter = g.orig
	}
	return repeat{g.orig, g.iter.Update(ctx)}
}

func (g repeat) Next(ctx context.Context) (interface{}, Generator) {
	if g.orig == nil {
		return StopIteration, nil
	}
	if g.iter == nil {
		g.iter = g.orig
	}
	x, iter := g.iter.Next(ctx)
	return x, repeat{g.orig, iter}
}

type GeneratorWithProb struct {
	Generator
	Prob float64
}

func (g GeneratorWithProb) Valid() bool { return g.Prob > 0 && g.Generator != nil }

type Choices []GeneratorWithProb

func (gs Choices) Update(ctx context.Context) Generator {
	out := make(Choices, 0, len(gs))
	for _, g := range gs {
		if g.Valid() {
			ng := g.Update(ctx)
			if ng != nil {
				out = append(out, GeneratorWithProb{ng, g.Prob})
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (gs Choices) Next(ctx context.Context) (interface{}, Generator) {
	n, s := 0, .0
	for _, g := range gs {
		if g.Valid() {
			n += 1
			s += g.Prob
		}
	}
	if n == 0 {
		return StopIteration, nil
	}
	t := rand.Float64() * s
	ngs := make(Choices, 0, n)
	var (
		x  interface{}
		ng Generator
		ok bool
	)
	for _, g := range gs {
		if !g.Valid() {
			continue
		}
		t -= g.Prob
		if !ok && t < 0 {
			ok = true
			x, ng = g.Next(ctx)
			if ng != nil {
				ngs = append(ngs, GeneratorWithProb{ng, g.Prob})
			}
		} else {
			ngs = append(ngs, g)
		}
	}
	if len(ngs) == 0 {
		return x, nil
	}
	return x, ngs
}

func TimeLimit(d time.Duration, g Generator) Generator {
	if g == nil || d <= 0 {
		return nil
	}
	return timeLimit{g, time.After(d)}
}

type timeLimit struct {
	inner Generator
	ch    <-chan time.Time
}

func (g timeLimit) Update(ctx context.Context) Generator {
	if g.inner == nil {
		return nil
	}
	ng := g.inner.Update(ctx)
	if ng == nil {
		return nil
	}
	return timeLimit{ng, g.ch}
}

func (g timeLimit) Next(ctx context.Context) (interface{}, Generator) {
	if g.inner == nil {
		return StopIteration, nil
	}
	select {
	case <-ctx.Done():
		return Pending, g
	case <-g.ch:
		return StopIteration, nil
	default:
		x, ng := g.inner.Next(ctx)
		if ng != nil {
			ng = timeLimit{ng, g.ch}
		}
		return x, ng
	}
}

func Stagger(d time.Duration, g Generator) Generator {
	if d <= 0 {
		return g
	}
	return StaggerFn(func() <-chan time.Time {
		return time.After(time.Duration(rand.Int63n(d.Nanoseconds() * 2)))
	}, g)
}

func StaggerFn(f func() <-chan time.Time, g Generator) Generator {
	if g == nil {
		return nil
	}
	if f == nil {
		return g
	}
	return stagger{g, nil, f}
}

type stagger struct {
	inner Generator
	ch    <-chan time.Time
	f     func() <-chan time.Time
}

func (g stagger) Update(ctx context.Context) Generator {
	if g.inner == nil {
		return nil
	}
	ng := g.inner.Update(ctx)
	if ng == nil {
		return nil
	}
	return stagger{ng, g.ch, g.f}
}

func (g stagger) Next(ctx context.Context) (interface{}, Generator) {
	if g.inner == nil {
		return StopIteration, nil
	}
	if g.ch == nil {
		g.ch = g.f()
	}
	select {
	case <-ctx.Done():
		return Pending, g
	case <-g.ch:
		x, ng := g.inner.Next(ctx)
		if ng != nil {
			ng = stagger{ng, g.f(), g.f}
		}
		return x, ng
	}
}
