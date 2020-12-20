package gen

import (
	"context"
	"math/rand"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func exhaust(g Generator) []interface{} {
	var (
		x  interface{}
		xs []interface{}
	)
	for {
		if g == nil {
			return xs
		}
		x, g = g.Next(context.TODO())
		xs = append(xs, x)
	}
}

func TestNone(t *testing.T) {
	require.Nil(t, None())
}

func TestSome(t *testing.T) {
	ctx := context.Background()

	t.Run("Nil", func(t *testing.T) {
		g := Some(nil)
		x, ng := g.Next(ctx)
		require.Nil(t, x)
		require.Nil(t, ng)
	})

	t.Run("Pending", func(t *testing.T) {
		g := Some(Pending)
		x, ng := g.Next(ctx)
		require.True(t, IsPending(x))
		require.Nil(t, ng)
	})

	t.Run("Value", func(t *testing.T) {
		g := Some(42)
		x, ng := g.Next(ctx)
		require.Equal(t, 42, x)
		require.Nil(t, ng)
	})

	t.Run("Func", func(t *testing.T) {
		var x interface{}
		g := Some(func() interface{} { return 42 })
		for i := 0; i < 5; i++ {
			x, g = g.Next(ctx)
			require.Equal(t, 42, x)
			require.NotNil(t, g)
		}
		g = Some(func(_ context.Context) interface{} { return 42 })
		for i := 0; i < 5; i++ {
			x, g = g.Next(ctx)
			require.Equal(t, 42, x)
			require.NotNil(t, g)
		}
		f := func() { t.Fatal("shouldn't be called") }
		x, g = Some(f).Next(ctx)
		require.Equal(t, reflect.ValueOf(f), reflect.ValueOf(x))
		require.Nil(t, g)
	})

}

func TestCons(t *testing.T) {
	g := Some(42)

	require.Nil(t, Cons(nil, nil))
	require.Equal(t, g, Cons(g, nil))
	require.Equal(t, g, Cons(nil, g))

	gg := Cons(g, g)
	require.Equal(t, []interface{}{42, 42}, exhaust(gg))
	require.Equal(t, []interface{}{42, 42, 42}, exhaust(Cons(gg, g)))
	require.Equal(t, []interface{}{42, 42, 42}, exhaust(Cons(g, gg)))
	require.Equal(t, []interface{}{42, 42, 42, 42}, exhaust(Cons(gg, gg)))
}

func TestSeq(t *testing.T) {
	for _, tt := range []struct {
		name string
		g    Generator
		r    []interface{}
	}{
		{"Empty", Seq(), nil},
		{"WithNil", Seq(42, nil, "foo"), []interface{}{42, "foo"}},
		{"Nested", Seq(Seq()), nil},
		{"Nested", Seq(Seq(42, "foo"), "bar"), []interface{}{42, "foo", "bar"}},
		{"Nested", Seq(42, Seq("foo", "bar")), []interface{}{42, "foo", "bar"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.r, exhaust(tt.g))
		})
	}
}

func TestMix(t *testing.T) {
	require.Nil(t, Mix())
	require.Nil(t, Mix(nil))

	xs := []interface{}{1, 2, 3, 4, 5}
	for i := 0; i < 3; i++ {
		g := Mix(xs...)
		ys := exhaust(g)
		shuffled := false
		for i := range xs {
			if xs[i] != ys[i] {
				shuffled = true
				break
			}
		}
		if shuffled {
			sort.Slice(ys, func(i, j int) bool { return ys[i].(int) < ys[j].(int) })
			require.Equal(t, xs, ys)
			return
		}
	}
	t.Fatal("shouldn't reach here")
}

func TestMap(t *testing.T) {
	id := func(x interface{}) interface{} { return x }
	pending := func(x interface{}) interface{} { return Pending }
	double := func(x interface{}) interface{} {
		if n, ok := x.(int); ok {
			return n * 2
		}
		return Pending
	}

	for _, tt := range []struct {
		name string
		g    Generator
		r    []interface{}
	}{
		{"Nil", Map(id, nil), nil},
		{"Pending", Map(pending, Seq(1, 2, 3)), []interface{}{Pending, Pending, Pending}},
		{"Double", Map(double, Seq(1, 2, 3)), []interface{}{2, 4, 6}},
		{"Double", Map(double, Seq(1, "oops", 3)), []interface{}{2, Pending, 6}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.r, exhaust(tt.g))
		})
	}
}

func TestFlatMap(t *testing.T) {
	id := func(x interface{}) Generator { return Some(x) }
	repeat := func(x interface{}) Generator { return Cons(Some(x), Some(x)) }
	even := func(x interface{}) bool {
		if n, ok := x.(int); ok {
			return n%2 == 0
		}
		return false
	}

	for _, tt := range []struct {
		name string
		g    Generator
		r    []interface{}
	}{
		{"Nil", FlatMap(id, nil), nil},
		{"Id", FlatMap(id, Seq(1, 2, 3)), []interface{}{1, 2, 3}},
		{"Repeat", FlatMap(repeat, Seq(1, 2)), []interface{}{1, 1, 2, 2}},
		{"Filter", Filter(even, Seq(1, 2, 3, 4)), []interface{}{2, 4}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.r, exhaust(tt.g))
		})
	}
}

func TestLimit(t *testing.T) {
	for _, tt := range []struct {
		name string
		g    Generator
		r    []interface{}
	}{
		{"Nil", Limit(3, None()), nil},
		{"Seq", Limit(0, Seq(1, 2)), nil},
		{"Seq", Limit(1, Seq(1, 2)), []interface{}{1}},
		{"Seq", Limit(3, Seq(1, 2)), []interface{}{1, 2}},
		{"Once", Once(Some(func() interface{} { return 42 })), []interface{}{42}},
		{"Limit", Limit(3, Some(func() interface{} { return 42 })), []interface{}{42, 42, 42}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.r, exhaust(tt.g))
		})
	}
}

func TestRepeat(t *testing.T) {
	for _, tt := range []struct {
		name string
		g    Generator
		r    []interface{}
	}{
		{"Nil", Limit(2, Repeat(nil)), nil},
		{"Repeat", Limit(3, Repeat(Some(1))), []interface{}{1, 1, 1}},
		{"Repeat", Limit(5, Repeat(Seq(1, 2))), []interface{}{1, 2, 1, 2, 1}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.r, exhaust(tt.g))
		})
	}
}

func TestChoices(t *testing.T) {
	ctx := context.Background()

	t.Run("Empty", func(t *testing.T) {
		g := Choices{}
		x, ng := g.Next(ctx)
		require.True(t, IsStopIteration(x))
		require.Nil(t, ng)
	})

	t.Run("Exhaust", func(t *testing.T) {
		g := Choices{
			{Seq(1, 3, 5), 1},
			{Seq(2, 4, 6), 1},
			{nil, 1},
			{Some(42), 0},
		}
		ys := exhaust(g)
		require.Len(t, ys, 6)
		ps1 := make([]int, 0, 3)
		ps2 := make([]int, 0, 3)
		for k, y := range ys {
			if y.(int)%2 == 0 {
				ps1 = append(ps1, k)
			} else {
				ps2 = append(ps2, k)
			}
		}
		require.True(t, sort.IntsAreSorted(ps1))
		require.True(t, sort.IntsAreSorted(ps2))
	})

	t.Run("Distribution", func(t *testing.T) {
		g := Limit(1000, Choices{
			{Some(func() interface{} { return 0 }), 2},
			{Some(func() interface{} { return 1 }), 3},
			{Some(func() interface{} { return 2 }), 5},
		})
		var cnts [3]float64
		for _, x := range exhaust(g) {
			cnts[x.(int)] += 1
		}
		require.InDelta(t, 200, cnts[0], 50)
		require.InDelta(t, 300, cnts[1], 50)
		require.InDelta(t, 500, cnts[2], 50)
	})

}

func TestTimeLimit(t *testing.T) {

	t.Run("Nil", func(t *testing.T) {
		require.Nil(t, TimeLimit(time.Second, nil))
	})

	t.Run("Exhaust", func(t *testing.T) {
		g := TimeLimit(time.Second, Limit(5, Repeat(Some(1))))
		require.Equal(t, []interface{}{1, 1, 1, 1, 1}, exhaust(g))
	})

	t.Run("Error", func(t *testing.T) {
		g := TimeLimit(time.Millisecond, Some(1))
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		x, g := g.Next(ctx)
		require.True(t, IsPending(x))
		require.NotNil(t, g)
		time.Sleep(time.Millisecond)
		x, g = g.Next(context.Background())
		require.True(t, IsStopIteration(x))
		require.Nil(t, g)
	})
}

func TestStagger(t *testing.T) {

	t.Run("Stagger", func(t *testing.T) {
		g := TimeLimit(time.Second, Stagger(5*time.Millisecond, Repeat(Some(1))))
		size := len(exhaust(g))
		require.InDelta(t, 200, size, 20)
	})

	t.Run("StaggerFn", func(t *testing.T) {
		ticks := time.NewTicker(time.Millisecond)
		defer ticks.Stop()
		g := TimeLimit(time.Second, StaggerFn(func() <-chan time.Time { return ticks.C }, Repeat(Some(1))))
		size := len(exhaust(g))
		require.InDelta(t, 1000, size, 50)
	})

}

func BenchmarkMap(b *testing.B) {
	ctx := context.Background()
	id := func(x interface{}) interface{} { return x }
	g := Map(id, Some(0))
	for i := 0; i < b.N; i++ {
		g.Next(ctx)
	}
}

func BenchmarkMapByFlatMap(b *testing.B) {
	Map := func(f func(x interface{}) interface{}, g Generator) Generator {
		return FlatMap(func(x interface{}) Generator { return Some(f(x)) }, g)
	}
	ctx := context.Background()
	id := func(x interface{}) interface{} { return x }
	g := Map(id, Some(0))
	for i := 0; i < b.N; i++ {
		g.Next(ctx)
	}
}

func BenchmarkStaggerRepeat(b *testing.B) {
	ctx := context.Background()
	g := Stagger(time.Millisecond, Repeat(Some(1)))
	for i := 0; i < b.N; i++ {
		_, g = g.Next(ctx)
	}
}

func BenchmarkStaggerRepeatFn(b *testing.B) {
	ctx := context.Background()
	ticks := time.NewTicker(time.Millisecond)
	defer ticks.Stop()
	g := StaggerFn(func() <-chan time.Time { return ticks.C }, Repeat(Some(1)))
	for i := 0; i < b.N; i++ {
		_, g = g.Next(ctx)
	}
}