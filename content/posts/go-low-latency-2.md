+++
date = 2022-11-19T19:12:32+03:00
title = "Go low latency patterns -- pointers"
description = "Patterns for go low-latency code"
slug = "go-low-latency-two"
tags = ["go-latency"]
categories = ["go", "performance"]
series = ["Go Low Latency"]
+++

Previously, we discussed common latency problems with
garbage collector, interfaces, generics and inlines.
In this post we'll talk about pointers usage in Go.
If you don't read the [first post](/posts/go-low-latency-one),
you may want to check it before reading this one.

This is the second post in "Go low latency patterns"
[series](/series/go-low-latency), based on the
"Low Latency Patterns" talk on "GopherCon Singapore 2023",
slides are available at: https://g4s8.github.io/gophercon-sg-2023

# Pointers

There are two main problems with pointers:
 - assigning a pointer to a struct field;
 - returning a pointer from method or function;

In both cases the pointer escapes to heap which affects latency as
we discussed in previous post.

## Assign to field

*As previously, we disable inlines optimization to avoid writing complex functions
and prefer simple examples for readability.*

Consider the following example:

```go
package main

type Child int

type Parent struct {
	Child *Child
}

func (p *Parent) SetChild(c *Child) {
	p.Child = c
}

func (p *Parent) SetChildDefault() {
	var c Child = 1
	p.Child = &c
}

func main() {
	var p Parent
	p.SetChildDefault()

	c := Child(2)
	p.SetChild(&c)
}
```

In both cases the child escapes to heap:
 - in `SetChildDefault`, the local-scope variable `c` escapes;
 - in `SetChild` the argument for this function escapes;

```txt
$ go build -gcflags '-m=1 -l'

# example.com
./main.go:9:7: p does not escape
./main.go:9:27: leaking param: c
./main.go:13:7: p does not escape
./main.go:14:6: moved to heap: c
./main.go:24:2: moved to heap: c
```

It's not possible to avoid allocation here.
Usually, this could be fixed by separating initialization and assignment.
We initialize all the data with allocation on application startup.
Then, we use it on the performance-critical path just by reading and writing to the memory allocated.

```go
package main

type Child int

type Parent struct {
	Child *Child
}

func NewParent() *Parent {
	return &Parent{
		Child: new(Child),
	}
}

func (p *Parent) SetChild(c Child) {
	*p.Child = c
}

func (p *Parent) SetChildDefault() {
	var c Child = 1
	*p.Child = c
}

func main() {
	p := NewParent() // initialization with allocation

	// assume performance critical code
	p.SetChildDefault()

	c := Child(2)
	p.SetChild(c)
}
```

In this case only initialization values are allocated on heap, but setters code
doesn't trigger allocations:

```txt
$ go build -gcflags '-m=1 -l'

# example.com
./main.go:10:9: &Parent{...} escapes to heap
./main.go:11:13: new(Child) escapes to heap
./main.go:15:7: p does not escape
./main.go:19:7: p does not escape
```

# Returns

The next important source of pointers heap escape is return of the pointer statements.

```go
package main

type Point struct {
	X, Y int
}

func NewPoint(x, y int) *Point {
	return &Point{x, y}
}

func main() {
	p := NewPoint(1, 2)
	println(p.X, p.Y)
}
```

Here, the value returned by `return &Point{x, y}` will be allocated on the heap.

To avoid this, we can add fluent setters for simpler initialization and delegate the responsibility
of constructing the "point" to the caller function.

```go
package main

type Point struct {
	X, Y int
}

func (p *Point) Set(x, y int) *Point {
	p.X = x
	p.Y = y
	return p
}

func main() {
	p := new(Point).Set(1, 2)
	println(p.X, p.Y)
}
```

In this example, the point `p` is allocated on the stack because the compiler can prove
that it won't be used after the function ends.

```asm
MOVUPS X15, 0x28(SP)		
LEAQ 0x28(SP), AX		
MOVL $0x1, BX			
MOVL $0x2, CX			
CALL main.(*Point).Set(SB)	
```

So if you want to avoid allocations on returning pointers, you may return either method receiver pointer,
like in the example above, or return any of function parameters.

# Examples

The types in the `math/big` package provide a good illustration of types designed for performance-critical code that can avoid memory allocations.
For instance, in the example below, none of the variables escape to the heap, and all computations on big integers are performed on the stack.

```go
package main

import "math/big"

func main() {
	one := new(big.Int).SetInt64(1)
	two := new(big.Int).SetInt64(2)
	three := new(big.Int).SetInt64(3)
	var sum big.Int
	sum.Add(&sum, one).Add(&sum, two).Add(&sum, three)
	println(sum.String())
}
```

## How to create similar types?

To create similar types that avoid allocations when working with pointers,
simply follow the rules from the sections above:
 - do not store external pointers in type fields;
 - do not return pointers from function/method scope;

Let's create a `SmallInt` type representing small integers with a method to add other small integers
to it by changing the state and a method to print it as a string.

The type definition may look like one item array:

```go
type SmallInt [1]int32
```

Then add a method to set its value and return pointer of itself:

```go
func (i *SmallInt) Set(x int32) *SmallInt {
	i[0] = x
	return i
}
```

Method for chaning its state:

```go
func (i *SmallInt) Add(x, y *SmallInt) *SmallInt {
	i[0] = x[0] + y[0]
	return i
}
```

And for printing itself as a string:

```go
func (i *SmallInt) String() string {
	return strconv.Itoa(int(i[0]))
}
```

Now let's use it:

```go
package main

import "strconv"

type SmallInt [1]int32

func (i *SmallInt) Set(x int32) *SmallInt {
	i[0] = x
	return i
}

func (i *SmallInt) Add(x, y *SmallInt) *SmallInt {
	i[0] = x[0] + y[0]
	return i
}

func (i *SmallInt) String() string {
	return strconv.Itoa(int(i[0]))
}

func main() {
	one := new(SmallInt).Set(1)
	two := new(SmallInt).Set(2)
	three := new(SmallInt).Set(3)
	var sum SmallInt
	sum.Add(&sum, one).Add(&sum, two).Add(&sum, three)
	println(sum.String())
}
```

If we build this code, we can see that there are no single allocation:

```txt
$ go build -gcflags '-m=1 -l'

# example.com
./main.go:7:7: leaking param: i to result ~r0 level=0
./main.go:12:7: leaking param: i to result ~r0 level=0
./main.go:12:24: x does not escape
./main.go:12:27: y does not escape
./main.go:17:7: i does not escape
./main.go:22:12: new(SmallInt) does not escape
./main.go:23:12: new(SmallInt) does not escape
./main.go:24:14: new(SmallInt) does not escape
```

Everething was computed on stack like for big integers.

# Dirty hacks

But what if need to bypass allocation in cases where we can't control the code?
For example if external library has methods which assign pointers to struct fields?

Let's assume our parent and child types are provided by such a library:

```go
type Child int

type Parent struct {
	Child *Child
}

func (p *Parent) SetChild(c *Child) {
	p.Child = c
}
```

And we want to set child field without allocation.
Actually it's possible to do with unsafe code but with some restrictions:

```go
func setChildUnsafe(p *Parent, c *Child) {
	p.Child = (*Child)(noescape(unsafe.Pointer(c)))
}

//go:nosplit
//go:nocheckptr
func noescape(p unsafe.Pointer) unsafe.Pointer {
	x := uintptr(p)
	return unsafe.Pointer(x ^ 0)
}

func main() {
	var p Parent
	var c Child
	setChildUnsafe(&p, &c)
}
```

I got the function `noescape` from Go source code internals. It breaks the dependency
between parameter pointer and returned pointer, so the escape analyzer can't determine
that these pointers has any relations.

To use it, just pass pointer of some type as a parameter, and cast the returned pointer back
to origin type. And thats all, there will be no allocations:

```txt
go build -gcflags '-m=1 -l'
# example.com
./main.go:11:7: p does not escape
./main.go:11:27: leaking param: c
./main.go:21:15: p does not escape
./main.go:15:21: p does not escape
./main.go:15:32: c does not escape
```

Everything is on stack:

```asm
; var p Parent
MOVQ $0x0, 0x18(SP)	
; var c Child
MOVQ $0x0, 0x10(SP)	
; setChildUnsafe(&p, &c)
LEAQ 0x18(SP), AX		
LEAQ 0x10(SP), BX		
CALL main.setChildUnsafe(SB)	
```

But be carefull with it.
**It could be dangerous** --- use only if the child object is not accessible outside of the parent’s stack frame.

Consider auto-cleanup of this field after using it in the same stack frame when child is assigned:

```go
func setChildUnsafe(p *Parent, c *Child) func() {
	p.Child = (*Child)(noescape(unsafe.Pointer(c)))
	return func() {
		p.Child = nil
	}
}

func dangerousOperation(p *Parent) {
	var c Child
	cleanup := setChildUnsafe(p, &c)
	defer cleanup()

	workWithParent(p)
}

func workWithParent(p *Parent) {
	// work with parent
}

func main() {
	var p Parent
	dangerousOperation(&p)
}
```

# Summary

In the second post of the 'Go Low Latency Patterns' series,
we delved into the use of pointers in Go, exploring how they can lead to heap escape and impact latency.
We discussed two primary issues: assigning a pointer to a struct field and returning a pointer from a method or function,
both of which can result in heap allocations.
We provided practical examples and solutions to avoid these allocations,
emphasizing the importance of separating initialization and assignment.
Additionally, we explored scenarios where external code or libraries introduce challenges and discussed 'dirty hacks' to address such situations,
with a cautionary note on their usage.
The post concludes with a reminder to be cautious when employing these hacks and highlights the value of
auto-cleanup to ensure safe operation within the same stack frame.

# Updates

Subscribe to get updates about next posts on this topic:
 - Telegram: [@g4s8_chan](https://t.me/g4s8_chan)
 - Twitter: [@kirill_che](https://twitter.com/kiryll_che)
