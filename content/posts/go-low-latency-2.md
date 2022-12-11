+++
date = 2022-11-19T19:12:32+03:00
title = "Go low latency patterns (II)"
description = "Patterns for go low-latency code"
slug = "go-low-latency-two"
tags = ["go-latency"]
categories = ["go", "performance"]
+++

This is the second post in "Go low latency patterns".
Previously, we discussed common latency problems with
garbage collector and patterns to avoid it.
In this post I'll show some advanced patterns and hacks for
writing low-latency code.
If you don't read the [first post](/posts/go-low-latency-one),
you may want to check it before reading this one.

## Object pools

Go standard library has a `sync.Pool` [API](https://pkg.go.dev/sync#Pool) for storing temporary objects in memory
to avoid redundant allocations. In a few words it keeps unused objects in memory and can remove it at any time,
but when you need to create a new object, this pool may reuse existing one instead of allocating memory for new object.
It doesn't help to get rid of heap allocations, but it may help to reduce the amount of allocations. Especially it helps
when you need to allocate quite bit amount of memory (e.g. image bitmap cache or temporary byte-buffers). It's not recommended
to use it for frequently allocated small short-lived objects, because overhead of using this pool will be large enough.

Example:
*Assuming you have a lot of images you need to process. You can create a pool to store image data to avoid allocations,
reset each image before drawing, and then draw it as needed.*

```go
var size = image.Rect(0, 0, 100, 100)

// entry point func
	pool := sync.Pool{
		New: func() any {
			return image.NewRGBA(size)
		},
	}

// processing func
        // get image from pool
	img := pool.Get().(*image.RGBA)
        // put if back on return
	defer pool.Put(img)

	// reset image - fill white
	draw.Draw(img, size, &image.Uniform{C: color.White}, image.ZP, draw.Src)

	// draw red rectangle
	draw.Draw(img, image.Rect(10, 10, 20, 20),
          &image.Uniform{C: color.RGBA{R: 255, A: 255}}, image.ZP, draw.Src)
```

## Short-living objects

For short-living objects you can construct a custom recycling queue using `chan` of structs, and customize
its parameters. It helps to avoid redundant allocations and moving to heap where possible.

Example:
*Assume, you need perform a fast processing of a big amount of byte buffers. A custom implementation
of chan based pool can be used for this.*

```go
type buffer []byte

type bufferPool struct {
	recycle chan buffer
}


func (p *bufferPool) get() (b buffer) {
        select {
        case b = <-p.recycle:
                atomic.AddUint64(&p.reuse, 1)
                return
        default:
        }
	return make(buffer, 1024*8)
}

func (p *bufferPool) put(b buffer) {
	select {
	case p.recycle <- b:
	default:
	}
}
```

And using buffers:
```go
        b := pool.get()
        processBuffer(b)
        pool.put(b)
```

I compared this implementation for reading `8*1024` bytes from `/dev/zero` in 100 goroutins, where each goroutine
reads it 100 times, another implementation didn't use pool. Baseline implementation just created new buffer every time
it was needed it:
```
BenchmarkStart/reuse-12         	     98	 12251340 ns/op	 833959 B/op	    212 allocs/op
BenchmarkStart/no_reuse-12      	     42	 28875223 ns/op	81938353 B/op	  10148 allocs/op
```

## Hack for keeping on stack

In [previous post](/posts/go-low-latency-one) I say that one struct is moving to heap when you set a pointer to this struct
as a field to another struct. What if I say that you can set a pointer to struct without moving child struct to heap?

This approach could be dangerous - you should be absolutely sure that child object has the same life-cycle as parent
object.

This magic function I copied from `runtime` package of standard library:
```go
//go:nosplit
//go:nocheckptr
func noescape(p unsafe.Pointer) unsafe.Pointer {
	x := uintptr(p)
	return unsafe.Pointer(x ^ 0)
}
```

To use it just pass pointer of target struct via this function and cast back to expected type:
```go
type Child struct {
	Val int
}

type Parent struct {
	C *Child
}

func (p *Parent) SetChildUnsafe(c *Child) {
	p.C = (*Child)(noescape(unsafe.Pointer(c)))
}

func main() {
	p := Parent{}
	c := Child{Val: 1}
	p.SetChildUnsafe(&c)
	useParent(&p)
}
```

If you run escape analysis on this code you can be surprised that `c` doesn't escape to heap. This hack
hides this `Child` reference from escape analysis and it's still allocated on stack, in spite of using it as
part of `Parent` struct. But **be careful** with this - if parent object lives longer than child, it cause
unpredictable behavior, child fields can be field with random data or replaced by new objects on stack.
