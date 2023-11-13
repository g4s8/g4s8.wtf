+++ 
date = 2022-09-10T14:19:32+03:00
title = "Go low latency patterns -- interfaces, generics and inlines"
description = "First part of go low-latency series: heap, stack, interfaces, generics and assembly. Analyze with assembly and objtool. Escape analysis. Memory optimizations. GoopherCon Singapore 2023."
slug = "go-low-latency-one" 
tags = ["go-latency", "go-interfaces", "go-generics"]
categories = ["go", "performance"]
series = ["Go Low Latency"]
keywords = ["Go", "low-latency", "Golang", "heap", "memory", "interface", "assembly"]

+++

If you are a Golang developer, you probably know that the Go language is often
chosen when low program latency is critical.
People opt for Go in such applications because it allows them to write programs
with predictable latency.

But unlike other languages like C or Rust, where a developer can imagine what machine code
will be compiled from some particular source code and approximately calculate the number of cycles to predict
execution time.
Go has some factors that affect latency implicitly. For example, the garbage collector (GC) is one of these factors.
In these posts, I aim to formulate implicit rules that help you write code with predictable latency.

It's the first post for "Go Low Latency" [series](/series/go-low-latency), based on the
"Low Latency Patterns" talk on "GopherCon Singapore 2023",
slides are available at: https://g4s8.github.io/gophercon-sg-2023

# Disclaimer

In many situations, the Go compiler is smart enough to produce optimized machine code.
And typically, you don't need to care about the internal representation of your data.

If you follow the tips from the
[Effective Go](https://go.dev/doc/effective_go) tutorial,
your code will perform well in 90% of cases.
In this post, I'll be discussing the other 10% - situations where your code must respond with **predictable** latency to events.
It's essential to keep in mind that the readability of code is often more important than a marginal (~10%) performance gain.

# Latency

According to [Wikipedia](https://en.wikipedia.org/wiki/Latency_%28engineering%29),
latency could be defined as:

> A time delay between the cause and the effect.

Of course, there are many possible sources of latency issues that we can't control,
such as hardware delays, system call latency, thread schedulers, and more.

But we can try to do our best at least at application level.
One of the main enemies of latency in an application is the Garbage Collector.

## Garbage Collector (GC)

The Go [garbage collector](https://go.dev/doc/gc-guide) (GC) affects the latency in
various ways, as cited from the [latency](https://go.dev/doc/gc-guide#Latency) section:
 1. Brief stop-the-world pauses when the GC transitions between the mark and sweep phases,
 2. Scheduling delays because the GC takes 25% of CPU resources when in the mark phase,
 3. User goroutines assisting the GC in response to a high allocation rate,
 4. Pointer writes requiring additional work while the GC is in the mark phase, and
 5. Running goroutines must be suspended for their roots to be scanned.

If we need to achieve predictable latency, we should avoid garbage collection during
the execution of critical code. This can be accomplished by not allocating memory
while this critical code is running because the GC's memory triggers
are based on the GOGC and GOMEMLIMIT parameters. In other words, the GC will not initiate
if no new heap memory is allocated for a certain period, allowing the critical code to execute.

The Go compiler uses complex logic to determine which variables should be moved to the heap or not,
and this logic may change from one compiler version to another.
In this article, I'm attempting to formulate empirical patterns and tips for escape analysis
that help avoid heap allocation for critical code sections.
I've tested these examples using Go version `1.21`.

## Tools

The most common tools for escape analysis is a:
 1. `go build` tool with `gcflags`, e.g.: `go build -gcflags '-m=2 -l' [package]`
 2. [pprof](https://go.dev/blog/pprof) tool used together with benchmark testing and
 `benhcmem` option.
 3. [ASM](https://go.dev/doc/asm) compiler to analyze assembly code:
 `go tool compile -S [file]` and `go tool objdump -s main.main -S example.com > main.go.s`
 4. Also, I really like [lensm](https://github.com/loov/lensm) app which helps to visualize
 assembly code and show the relation between it and lines of source code.

I'll use the output of these tools in my examples.


# Interfaces

When dealing with interface function parameters, be careful --- arguments for these parameters
are often moved to the heap before being passed to the callee function.

However, it can be challenging to determine precisely when an argument is moved to the heap and when it is not.
Even if escape analysis reports that a variable has been moved to the heap, it doesn't guarantee that it will always happen.
This ambiguity arises because the allocation logic depends on two components: the compiler and the runtime.
Escape analysis is integrated into the compiler to assist in deciding whether a variable should be moved to the heap.
However, during the application's runtime, the runtime may decide not to move it to the heap.
Therefore, it's advisable to consider the escape analysis output as a hint of where allocation may potentially occur.

When you have a function with an interface parameter, and the caller function passes an argument to the callee function,
the compiler generates code to convert this argument into an internal representation of interface values.
This internal representation consists of a two-word data structure: one word is a pointer to the argument value,
and the other word is a pointer to metadata (itable).
The metadata structure contains information about the argument's type and a function table for dynamic dispatch.
In some cases, the first pointer may contain the actual value itself (not a pointer),
but only if the compiler can prove that the value size is less or equal to the word length.

### Example

*For now, let's assume we disable inline optimizations with `go build -gcflags='-l'`, we'll discuss inlines a bit later.*

Now let's see the very small example:

```go
import "fmt"

func main() {
	var x1 int = 1
	fmt.Println(x1)
}
```

The code above produces this assembly code:
```asm
MOVL $0x1, AX			
CALL runtime.convT64(SB)	
LEAQ 0x6e9b(IP), CX		
MOVQ CX, 0x18(SP)		
MOVQ AX, 0x20(SP)		
LEAQ 0x18(SP), AX		
MOVL $0x1, BX			
MOVQ BX, CX			
NOPL 0(AX)			
CALL fmt.Println(SB)	
```

And here we can see what happens actually. Let's go step by step.

1. To pass variable `x1` with `1` value as `Println` argument
it is passed through `runtime.convT64` function, which takes `uint64` value
as a parameter and returns pointer for this value (stored in `AX`):
```asm
MOVL $0x1, AX			
CALL runtime.convT64(SB)	
```

2. Then it loads type information, and store it and the pointer to argument variable at stack
with offset (create interface value internal representation):
```asm
LEAQ 0x6e9b(IP), CX		
MOVQ CX, 0x18(SP)		
MOVQ AX, 0x20(SP)		
```

3. Set varargs length to `1`:
```asm
MOVL $0x1, BX			
MOVQ BX, CX			
```

4. Call `Println`:
```asm
CALL fmt.Println(SB)	
```

One interesting point here is that compiler was not able to prove that the argument value is less
than one word size and it didn't store it as value in interface structure but still used pointer.

If we compile this example with escape analysis output enabled `go build -gcflags='-l -m'`, we can see
that it reports that `x1` variable is escaped to heap:

```txt
./main.go:7:13: ... argument does not escape
./main.go:7:14: x1 escapes to heap
```

## Benchmarking

Indeed, compiler can't prove that the pointer of this variable will not be used later after passing it to
`Println` function. But if we make primitive benchmark for this case:

```go
package main

import (
	"fmt"
	"testing"
)

func BenchmarkPrintln(b *testing.B) {
	for i := 0; i < b.N; i++ {
		var x int = 1
		fmt.Println(x)
	}
}
```

We can suddenly discover that this case doesn't produce any allocation:
```txt
$ go test -bench=. -benchmem .

    463819	     2381 ns/op	      0 B/op	      0 allocs/op
```

What?!
Escape analysis output says that the value is moved to the heap, but memory benchmark didn't detect it?
How could this happen?

The answer lies in the runtime. Allocations in this example depend on the variable's value.
Let's see what happens if we change the value in our benchmark:

```go
package main

import (
	"fmt"
	"testing"
)

func BenchmarkPrintln(b *testing.B) {
	for i := 0; i < b.N; i++ {
		var x int = 256
		fmt.Println(x)
	}
}
```

And now when we run tests:
```txt
$ go test -bench=. -benchmem .

    455181	     2337 ns/op	      8 B/op	      1 allocs/op
```

So the new memory is allocated for `256` value but not for `1`.
But the assembly code is exactly the same for both variables,
it means that something different happens in runtime. The only
candidate here is the `runtime.convT64` function.

### The runtime and numbers cache

Here is the implementation of `convT64` function. Just to remind you:
it takes the `uint64` value and returns a pointer for it.

```go
func convT64(val uint64) (x unsafe.Pointer) {
	if val < uint64(len(staticuint64s)) {
		x = unsafe.Pointer(&staticuint64s[val])
	} else {
		x = mallocgc(8, uint64Type, false)
		*(*uint64)(x) = val
	}
	return
}
```

As you can see it has two branches:
 1. First is called when the parameter value is less then `staticuint64s`
 array length (surprise: it's 256). And it just takes the pointer of some value
 in this array by parameter index;
 2. Another is called otherwise, it allocates new memory, store the value in this
 memory, returns a pointer for the memory.

The array `staticuint64s` looks like this:

```go
// staticuint64s is used to avoid allocating in convTx for small integer values.
var staticuint64s = [...]uint64{
	0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
    // ...
	0xf8, 0xf9, 0xfa, 0xfb, 0xfc, 0xfd, 0xfe, 0xff,
}
```

So it's just a numbers from `0x00` to `0xff`. It's used to avoid allocations
in runtime for small numbers, this is why our `1` variable was not allocated
on heap but `256` (it's `0xff + 1`) was allocated. I hope now it's clear
about allocations and the joint efforts of runtime and compiler in this topic.

### Small interface arguments

As mentioned previously, the compiler may decide to keep the value
in interface structure instead of taking a pointer for it if it can prove
that the value is less than or equal one word size.
However, in the previous example, this behavior did not occur for some reason.
It would be beneficial to investigate why it did not happen. At the moment, I don't have an explanation for this.
But if we change the code from:

```go
package main

import "fmt"

func main() {
	var x1 int = 15
	fmt.Println(x1)
}
```

To:

```go
package main

import "fmt"

func main() {
	fmt.Println(int(15))
}
```

Now the compiler manages to prove it,
and the value is loaded directly from binary memory without any `convT64` calls:

```asm
MOVUPS X15, 0x18(SP)  ; clear the stack
LEAQ 0x6ea5(IP), DX   ; load `15` value from program memory
MOVQ DX, 0x18(SP)     ; store it in interface value 1st word
LEAQ 0x36f71(IP), DX  ; load type info
MOVQ DX, 0x20(SP)     ; store type info as 2nd word
LEAQ 0x18(SP), AX
MOVL $0x1, BX
MOVQ BX, CX
CALL fmt.Println(SB)
```

## A summary of escape analysis

We have just seen that the escape analysis of interface parameters can be tricky.
Even if the compiler's output indicates that the value escapes to the heap,
it only means that the value **may** escape to the heap.
Sometimes, the runtime may decide not to move it to the heap,
and sometimes, the compiler may optimize it and avoid calling `convT**` functions to extract pointers.
Therefore, always check your specific case with benchmarks to confirm whether the argument value is actually allocated.
Perhaps you don't even need to optimize it.

# Avoid allocations

First, try to avoid allocations only **if you have them and if you need to**.
Typically, zero-allocation code has a worse design when compared to simple code without zero-allocation considerations.

There are a few possible techniques. Let's start with an example:

```go
package main

type Inter interface { // like fmt.Stringer
	Int64() int64
}

type inter64 int64 // implementation

func (i inter64) Int64() int64 {
	return int64(i)
}

//go:noinline
func toInt(i Inter) int64 {
	return i.Int64()
}

func main() {
	x := inter64(256)
	_ = toInt(x)
}
```

Here we can see allocation of `x` via `i` param:

```txt
$ go build -gcflags '-m=2 -l'

# example.com
./main.go:14:12: parameter i leaks to {heap} with derefs=0:
./main.go:14:12:   flow: {heap} = i:
./main.go:14:12:     from i.Int64() (call parameter) at ./main.go:15:16
./main.go:14:12: leaking param: i
./main.go:20:12: x escapes to heap:
./main.go:20:12:   flow: {heap} = &{storage for x}:
./main.go:20:12:     from x (spill) at ./main.go:20:12
./main.go:20:12:     from toInt(x) (call parameter) at ./main.go:20:11
./main.go:20:12: x escapes to heap
<autogenerated>:1: parameter .this leaks to {heap} with derefs=0:
<autogenerated>:1:   flow: {heap} = .this:
<autogenerated>:1:     from .this.Int64() (call parameter) at <autogenerated>:1
```

Indeed, this variable is converted with `convT64` like in previous example:

```asm
MOVL $0x100, AX					
CALL runtime.convT64(SB)			
MOVQ AX, BX					
LEAQ go:itab.main.inter64,main.Inter(SB), AX	
CALL main.toInt(SB)	
```

## Exact type parameters

The simplest solution here is to avoid using interfaces:

```go
func toInt64(i inter64) int64 {
	return i.Int64()
}

func main() {
	x := inter64(256) // MOVL $0x100, AX
	_ = toInt64(x)    // CALL main.toInt64(SB)
}
```

## Generic functions

Certain variables may not be moved to the heap for functions with generic parameters:

```go
func toIntGeneric[T Inter](i T) int64 {
	return i.Int64()
}

func main() {
	x := inter64(256)
	_ = toIntGeneric(x)
}
```

Here, we've defined a new generic function with type `T`, where `T` can be any type that implements the Inter interface.
Now, let's compile it:

```txt
$ go build -gcflags '-m=2 -l'

# example.com
<autogenerated>:1: parameter .this leaks to {heap} with derefs=0:
<autogenerated>:1:   flow: {heap} = .this:
<autogenerated>:1:     from .this.Int64() (call parameter) at <autogenerated>:1
```

No allocations --- that's a win. However, it's not as straightforward as it sounds.

```asm
LEAQ main..dict.toIntGeneric[main.inter64](SB), AX
MOVL $0x100, BX
CALL main.toIntGeneric[go.shape.int64](SB)
```

When we examine the assembly code, we notice a few interesting things:
 1. our argument was passed via the `BX` register without heap allocation;
 2. we call a function named `toIntGeneric[go.shape.int64]`;
 3. some data from `toIntGeneric[main.inter64]` is loaded into the `AX` register;

Let's address these points one by one. The first point is what we are aiming for, so it's expected.

The function name includes the `go.shape.int64` suffix because the Go compiler generates actual generic code only for different GC shapes,
not for each type. GC-shapes differ for types that have varying sizes, alignments, and whether or not they contain pointers.

From the generics proposal called 
[GC Shape Stenciling](https://github.com/golang/proposal/blob/master/design/generics-implementation-gcshape.md)
we can get the definition of GC-shape as
> The GC shape of a type means how that type appears to the allocator / garbage collector.
> It is determined by its size, its required alignment, and which parts of the type contain a pointer.

Our type `inter64` appears to the allocator as just an `int64` value, which doesn't require heap allocation.
Therefore, this function accepts values of int64-shaped types as if it were:

```go
func toIntGenericInt64(i inter64)
```

The third point relates to this instruction, which loads type information for `inter64` into the `AX` register:

```asm
LEAQ main..dict.toIntGeneric[main.inter64](SB), AX
```

The compiler performs this action because function calls methods on generic types via dynamic dispatch.
This approach has two drawbacks for generic functions:
 - calling generic functions becomes more expensive due to dynamic dispatch;
 - it's not possible to apply different optimizations to this call;

The first point has a relatively lower impact on performance compared to allocation, which generics can help avoid.
However, the second point may be a more serious issue. For example,
if a function with an interface type parameter could be inlined and optimized, a generic function cannot be inlined.
In some cases, a function with an interface type parameter may perform significantly better
for performance after optimizations than a generic function.

And this leads us to the next topic.

# Inlines

When a function call with an interface parameter is inlined, it bypasses virtual table lookup,
and the compiler may not move its arguments to the heap.

Let's seethe compiler optimizations 
[documentation](https://github.com/golang/go/wiki/CompilerOptimizations#escape-analysis-and-inlining):

Only short and simple functions are inlined. To be inlined a function must conform to the rules:
 - function should be simple enough, the number of AST nodes must less than the budget (80);
 - function doesn't contain complex things like closures, defer, recover, select, etc;
 - function isn't prefixed by go:noinline;
 - function isn't prefixed by go:uintptrescapes, since the escape information will be lost during inlining;
 - function has body;

And now try to create a function with interface type params for inlining:

```go
package main

type Calc interface {
	Add(int) int
}

type calcInt int

func (c *calcInt) Add(n int) (sum int) {
	sum = int(*c) + int(n)
	*c = calcInt(sum)
	return
}

func main() {
	var c1 calcInt
	_ = sum(&c1, 1, 2, 3)
}

func sum(calc Calc, vals ...int) (sum int) {
	for _, val := range vals {
		sum = calc.Add(val)
	}
	return
}
```

It looks pretty simple:
 - there is a `calcInt` type of `int` with method `Add(int) int`
 which adds a number to itself and returns the value. The receiver
 is a pointer type;
 - `Calc` interface for function parameter corresponding to `calcInt` type;
 - a method `sum` which accepts implementation of `Calc` and integers as vararg
 to calculate a sum of integer using `Calc`;
 - `main` func to call it all together;

If we build it with inlines enabled:

```txt
$ go build -gcflags '-m=1'

# example.com
./main.go:9:6: can inline (*calcInt).Add
./main.go:20:6: can inline sum
./main.go:17:9: inlining call to sum
./main.go:17:9: devirtualizing calc.Add to *calcInt
./main.go:9:7: c does not escape
./main.go:17:9: ... argument does not escape
./main.go:20:10: leaking param: calc
./main.go:20:21: vals does not escape
```

We can see that the call was inlined. Let's check assembly:

```asm
; initialize c1 variable with zero
MOVQ $0x0, 0x10(SP)	
; initialize varargs with 1, 2, 3
MOVUPS X15, 0x20(SP)	
MOVUPS X15, 0x28(SP)	
MOVQ $0x1, 0x20(SP)	
MOVQ $0x2, 0x28(SP)	
MOVQ $0x3, 0x30(SP)	
XORL AX, AX		
; call to sum inlined
; range loop head
JMP 0x457740		
MOVQ AX, 0x18(SP)	
MOVQ 0x20(SP)(AX*8), BX	
; calling Add method on calcInt implementation directly
LEAQ 0x10(SP), AX		
CALL main.(*calcInt).Add(SB)	
; range loop tail
MOVQ 0x18(SP), AX	
INCQ AX			
NOPW			
CMPQ AX, $0x3		
JL 0x457722
```

As we can see, the main function after inlining calls `Add` method on `calcInt`
implementation of `Calc` directly. So we have no allocations here and
no dynamic dispatch.

What if we change the `sum` implementation to accept generic type parameter?

```go
func main() {
	var c1 calcInt
	_ = sum(&c1, 1, 2, 3)
}

func sum[T Calc](calc T, vals ...int) (sum int) {
	for _, val := range vals {
		sum = calc.Add(val)
	}
	return
}
```

Build it:

```txt
$ go build -gcflags '-m=1'

# example.com
./main.go:9:6: can inline (*calcInt).Add
./main.go:20:6: can inline sum[go.shape.*uint8]
./main.go:17:9: inlining call to sum[go.shape.*uint8]
./main.go:20:6: inlining call to sum[go.shape.*uint8]
./main.go:9:7: c does not escape
./main.go:16:6: moved to heap: c1
./main.go:17:9: ... argument does not escape
```

The function itself was inlined, but what about parameter calls?

```asm
; allocate c1 variable on heap
LEAQ 0x4eab(IP), AX		
CALL runtime.newobject(SB)	
MOVQ AX, 0x30(SP)		
; initialize varargs
MOVUPS X15, 0x18(SP)	
MOVUPS X15, 0x20(SP)	
MOVQ $0x1, 0x18(SP)	
MOVQ $0x2, 0x20(SP)	
MOVQ $0x3, 0x28(SP)	
XORL CX, CX		
; function call inlined
; range loop head
JMP 0x457751		
MOVQ CX, 0x10(SP)	
MOVQ 0x18(SP)(CX*8), BX	
; call `calc.Add(val)` dynamically
LEAQ main..dict.sum[*main.calcInt](SB), DX	
LEAQ 0xffffff7e(IP), SI				
CALL SI						
; range loop middle
MOVQ 0x10(SP), CX	
INCQ CX			
; sum = <result of SI call>
MOVQ 0x30(SP), AX	
; range loop tail
CMPQ CX, $0x3		
JL 0x45772a		
```

Unlike calls with interface parameters, we can see that the implementation call was not inlined.
Additionally, there is an allocation for the argument because the method has a pointer receiver,
resulting in a GC shape of `*calcInt`. This leads the allocator to decide to store it in the heap.

# Pros and Cons

Here's a summary table for the three different approaches discussed in this post:
 - function with exact type parameters;
 - function with interface type parameters;
 - function with generic type parameters derived from interface type;

| Type         | Allocs        | Dyn. dispatch    | Inline     |
|:-------------|:--------------|:-----------------|:-----------|
| Exact        | No            | No               | Yes        |
| Interface    | Inline        | Inline           | Yes        |
| Generics     | Shapes        | Yes              | No         |

In the table above,
"exact" type parameters can help avoid allocations entirely.
They have no dynamic dispatch and can be optimized by the compiler.
However, this approach may lead to less desirable design decisions in some cases.

Functions with interface type parameters can be inlined with the implementation of the interface.
This allows for optimization to avoid dynamic dispatch and argument allocations, if they exist.
It provides more design flexibility.

Generic parameters of certain GC shapes are not moved to the heap.
However, functions with these parameters have dynamic dispatch for method calls on generic-typed parameters,
which prevents implementation calls from being inlined.
From a design perspective, it is similar to interface parameters or sometimes even better.

# Summary

First, verify if you have a problem. Even if escape analysis indicates heap allocation, it may not be your actual case.
it may be not your case.

Small functions with interface parameters can often be inlined with the implementation.
If this occurs, manual optimization may not be necessary.

Generic functions with value types are your allies, as these GC shapes are usually not moved to the heap.

As a last resort, consider implementing different functions for each type to avoid allocation in performance-critical paths.
Examples like [github.com/rs/zerolog](https://github.com/rs/zerolog) can provide guidance.

Don't hesitate to redesign your code when needed.
Adjusting the design of implementation can also improve performance without significant drawbacks.
For instance, using small functions in interface implementations can help the compiler inline and optimize them.

# Updates

Subscribe to get updates about next posts on this topic:
 - Telegram: [@g4s8_chan](https://t.me/g4s8_chan)
 - Twitter: [@kirill_che](https://twitter.com/kiryll_che)

# References

 - [Effective Go](https://go.dev/doc/effective_go)
 - [Garbage Collector](https://go.dev/doc/gc-guide)
 - [pprof](https://go.dev/blog/pprof)
 - [Go assembly](https://go.dev/doc/asm)
 - [lensm](https://github.com/loov/lensm) helps to visualize assembly
 - [Generic implementation](https://github.com/golang/proposal/blob/master/design/generics-implementation-gcshape.md)
 - [Compiler optimizations](https://github.com/golang/go/wiki/CompilerOptimizations):
