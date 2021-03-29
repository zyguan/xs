package rule

import "fmt"

func echo(xs ...interface{}) { fmt.Printf("%+v\n", xs) }

func ExampleRule1() {
	r := Seq(1, 2, Empty())
	Walk(r, echo)
	// Output: [1 2]
}

func ExampleRule2() {
	r := OneOf(Empty(), 1, 2)
	Walk(r, echo)
	// Output:
	// []
	// [1]
	// [2]
}

func ExampleRule3() {
	r := Seq(
		OneOf(Empty(), 1),
		OneOf(2, 3),
		OneOf(4, Empty()),
	)
	Walk(r, echo)
	// Output:
	// [2 4]
	// [2]
	// [3 4]
	// [3]
	// [1 2 4]
	// [1 2]
	// [1 3 4]
	// [1 3]
}
