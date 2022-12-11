+++ 
date = 2022-09-10T14:19:32+03:00
title = "Go low latency patterns (I)"
description = "Programming patterns mostly empirical for low latency applications"
slug = "go-low-latency-one" 
tags = ["go-latency"]
categories = ["go", "performance"]
+++

If you are a Golang developer, you most probably know that
Go language is frequently chosen when program latency is critical.
People choose Go for this kind of applications because it allows to
write programs with predictable latency. But unlike other languages
like C or Rust, where developer can imagine what machine code will be compiled from
some particular source code and approximately calculate the amount of cycles
to predict execution time, Go has some factors which affects latency
implicitly, e.g. garbage collector (GC). In these posts I'm trying to
formulate implicit rules helping to write the code with predictable
latency.

### Disclaimer

In many situations the Go compiler is smart enough to produce
optimized machine code. And mostly you don't need to care about
internal representation of your data. If you follow the tips from
[Effective Go](https://go.dev/doc/effective_go) tutorial, your code
will perform fast in 90% of cases. In this post I'm talking about
other 10% - sometimes you meet the requirements where the code should respond to an
event with **predictable** latency. Also, keep in mind, that the readability of code
is usually more important than ~10% of performance gain.

### Garbage Collector

The Go [garbage collector](https://go.dev/doc/gc-guide) (GC) affects the latency in a
different ways (citation from [latency](https://go.dev/doc/gc-guide#Latency) section):
 1. Brief stop-the-world pauses when the GC transitions between the mark and sweep phases,
 2. Scheduling delays because the GC takes 25% of CPU resources when in the mark phase,
 3. User goroutines assisting the GC in response to a high allocation rate,
 4. Pointer writes requiring additional work while the GC is in the mark phase, and
 5. Running goroutines must be suspended for their roots to be scanned.

So if we need to have a predictable latency we should avoid garbage collection during
execution of critical code. It could be achieved if we don't allocate anything
while this critical code is executing, because GC has memory triggers for execution
based on `GOGC` and `GOMEMLIMIT` parameters. In other words, GC will not start
if we don't allocate new heap memory for some period of time needed for critical code
to be executed.

Go compiler uses complex logic to decide
which variable should be moved to heap or not, and this logic could change from one
compiler version to another. In this article I'm trying to formulate
empirical patterns and tips for escape analysis which helps to avoid
heap allocation for critical code section. I've tested these examples with `1.19` Go
version.

### Tools

The most common tools for escape analysis is a:
 1. `go build` tool with `gcflags`, e.g.: `go build -gcflags '-m=2 -l' [package]`
 2. [pprof](https://go.dev/blog/pprof) tool used together with benchmark testing and
 `benhcmem` option.
 3. [ASM](https://go.dev/doc/asm) compiler to analyze assembly code:
 `go tool compile -S [file]`.

I'll use the output of these tools in my examples.

## No allocation patterns

This is a list of programming patterns in Go which helps to avoid allocations
in some critical code section. Most of them were found empirically with
escape heap analysis, and some of them are just logical patterns, e.g.
you can understand that the caller function can't access the stack callee function,
so the result will be moved to heap if caller needs to access it.

### Don't use interfaces

If a function accepts interface as argument, then this argument parameter will
be moved to heap (only if argument is used inside this function).

Example:
```go
package main

type fooer interface {
	foo()
}

type foo int

func (f foo) foo() {
	print("foo")
}

func main() {
	var f foo
	printFoo(f)
}

func printFoo(f fooer) {
	f.foo()
}
```

The function `printFoo` accepts interface as argument,
integer type `foo` moved to heap before calling `printFoo`:
```
printinterface/main.go:17:15: parameter f leaks to {heap} with derefs=0:
printinterface/main.go:17:15:   flow: {heap} = f:
printinterface/main.go:17:15:     from f.foo() (call parameter) at printinterface/main.go:18:7
printinterface/main.go:17:15: leaking param: f
printinterface/main.go:14:10: f escapes to heap:
printinterface/main.go:14:10:   flow: {heap} = &{storage for f}:
printinterface/main.go:14:10:     from f (spill) at printinterface/main.go:14:10
printinterface/main.go:14:10:     from printFoo(f) (call parameter) at printinterface/main.go:14:10
printinterface/main.go:14:10: f escapes to heap
```

To avoid this declare argument types explicitly in methods:
```diff
- func printFoo(f fooer) {
+ func printFoo(f foo) {
```

### Returning pointers from functions

If a function returns a pointer to a new value created inside the function,
this value will be moved to heap.

Example:
```go
package main

type foo struct {
	x int
}

func main() {
	_ = newFoo(1)
}

func newFoo(x int) *foo {
	return &foo{x}
}

// returnptr/main.go:12:9: &foo{...} escapes to heap
```

Go compiler can't create this `foo` object on stack because when the function
returns, the stack is popped and all values on stack becomes invalid. This value
should be moved to heap to allow function caller to access this object.

This could be fixed by two ways:
 1. Return value, not pointer.
 2. Pass value to function as argument and initialize it in function.

First solution:
```diff
- func newFoo(x int) *foo {
- 	return &foo{x}
- }
+ func newFoo(x int) foo {
+ 	return foo{x}
+ }
```

Second solution:
```go
package main

type foo struct {
	x int
}

func main() {
	_ = makeFoo(new(foo), 1)
}

func makeFoo(f *foo, x int) *foo {
	f.x = x
	return f
}
```

In this case returning pointer doesn't escape:
```
returnptr/main.go:11:14: leaking param: f to result ~r0 level=0
returnptr/main.go:8:17: new(foo) does not escape
```

### Setting new field values to argument pointer fields

Setting new values for argument pointer field moves this value to the heap.

```go
package main

type foo struct {
	x int
}

type bar struct {
	f *foo
}

func main() {
	var b bar
	b.set()
}

func (b *bar) set() {
	b.f = &foo{}
}

// fields/main.go:16:7: b does not escape
// fields/main.go:17:8: &foo{} escapes to heap
```

It's not important that we create new `foo{}` object here,
if we pass it from `set` caller stack it will be moved to heap anyway:

```go
package main

type foo struct {
	x int
}

type bar struct {
	f *foo
}

func main() {
	var b bar
	var f foo
	b.set(&f)
}

func (b *bar) set(f *foo) {
	b.f = f
}

// fields/main.go:17:7: b does not escape
// fields/main.go:17:19: leaking param: f
// fields/main.go:13:6: moved to heap: f
```

How to fix this? It depends... One solution could be to use value instead of pointer:
```go
package main

type foo struct {
	x int
}

type bar struct {
	f foo
}

func main() {
	var b bar
	var f foo
	b.set(f)
}

func (b *bar) set(f foo) {
	b.f = f
}
```

Or moving fields assignments to the same stack as object allocation, and copy
fields from one object to another without assignments:
```go
package main

type foo struct {
	x int
}

type bar struct {
	f *foo
}

func main() {
	var b bar
	b.f = &foo{}
	b.set(&foo{x: 1})
}

func (b *bar) set(f *foo) {
	b.f.x = f.x
}

// fields/main.go:17:7: b does not escape
// fields/main.go:17:19: f does not escape
// fields/main.go:13:8: &foo{} does not escape
// fields/main.go:14:8: &foo{...} does not escape
```

### Slice, map, channel pointer value types

If slice or map value or channel type
is not a pointer type, it could be set via function without moving to heap,
and it's moving to heap for pointer types:
```go
package main

func main() {
	s := make([]*int, 10)
	x := 5
	setVal(s, 0, &x)
}

func setVal(s []*int, i int, val *int) {
	s[i] = val
}

// setslice/main.go:9:13: s does not escape
// setslice/main.go:9:30: leaking param: val
// setslice/main.go:5:2: moved to heap: x
// setslice/main.go:4:11: make([]*int, 10) does not escape
```

```go
package main

type foo struct {
	ch chan int
}

func main() {
	f := &foo{ch: make(chan int)}
	send(f, 42)
}

func send(f *foo, x int) {
	f.ch <- x
}

// sendchan/main.go:12:11: f does not escape
// sendchan/main.go:8:7: &foo{...} does not escape
```

### Allocating big slices

When `make` slice size parameter is big enough, the entire slice
could be moved to heap:
```go
_ = make([]int, 100)    // create on stack
_ = make([]int, 100000) // move to heap
```

Slice will be also moved to heap in case of dynamic size parameter:
```go
package main

func main() {
	_ = newSlice(1)
}

func newSlice(size int) []int {
	return make([]int, size)
}

// array/main.go:4:6: moved to heap: x
```

By the way, arrays will be also moved to heap in case of big size,
but the size threshold is higher then slice threshold:
```go
package main

func main() {
	var x [10000000]int
	_ = x
}

// array/main.go:4:6: moved to heap: x
```

## To be continued

In next posts of this series I'll show other patterns related to
low latency and memory allocations.
