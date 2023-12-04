+++
date = 2023-11-28T12:45:52+04:00
title = "A few notes on cache lines in Go"
description = """
Explore the subtle yet impactful world of cache lines in GoLang.
Learn how strategic padding in structs can optimize performance in concurrent programming.
Discover benchmark results and gain insights into when and why optimizing cache lines matters in your Go applications."
"""
slug = ""
tags = ["go"]
categories = ["go", "performance"]
series = ["Go Low Latency"]
+++

Let's continue performance posts series.
In concurrent programming with Go, the efficiency can be impacted by issues like cache lines and false sharing
when multiple goroutines access shared data. This blog explores these challenges and demonstrates how adding
padding to structs can enhance performance in some cases.

# Disclaimer

While the performance gains demonstrated through cache line optimization techniques can be significant in certain scenarios,
it's essential to exercise caution and strike a balance in your optimization efforts.
Over-optimization, especially for CPU cache performance, may not be necessary for every application.

Go is designed with simplicity and efficiency in mind, and in many cases, the default behavior provides satisfactory performance.
Only embark on optimization journeys when your specific use case demands it,
and remember that readability and maintainability of your code should not be sacrificed for marginal performance gains.

It's crucial to profile your application, identify actual bottlenecks, and prioritize optimizations accordingly.
Keep in mind that optimizing for cache lines is a nuanced practice and might not be a common requirement in everyday Go development.

Use these techniques judiciously, considering the trade-offs and the specific needs of your application.
Always measure and validate the impact of optimizations through thorough testing before incorporating them into production code.

# Understanding the Basics

Before diving into the examples, let's briefly discuss cash lines and false sharing.
A [cache line](https://cseweb.ucsd.edu/classes/su07/cse141/cache-handout.pdf)
is a small block of memory typically loaded into the processor's cache.
[False sharing](https://en.wikipedia.org/wiki/False_sharing)
occurs when two or more threads update variables that reside on the same cache line,
leading to unnecessary synchronization and degraded performance.

# Examples and benchmarks

Let's benchmark the performance of writing to neighbors small fields on one struct.
In this example we'll see the scenario where two variables in memory are updated from
two goroutines.

Here is the benhcmark test utils:

```go
type Container interface {
	AddFoo(x uint64)
	AddBar(y uint64)
}

func mutator(c Container, f func(Container, uint64), n int, wg *sync.WaitGroup) {
	for i := 0; i < n; i++ {
		f(c, uint64(i))
	}
	wg.Done()
}

func containerBenchmarker(c Container) func(*testing.B) {
	return func(b *testing.B) {
		c.Reset()
		var wg sync.WaitGroup
		wg.Add(2)
		go mutator(c, Container.AddFoo, b.N, &wg)
		go mutator(c, Container.AddBar, b.N, &wg)
		wg.Wait()
	}
}
```

In this benchmark we write `uint64` values from two goroutines.
Now let's try it with different implementations:

----

Example 1: `ContainerPlain` - struct with two fields without padding

```go
type ContainerPlain struct {
	foo, bar uint64
}

func (c *ContainerPlain) AddFoo(x uint64) {
	c.foo += x
}

func (c *ContainerPlain) AddBar(x uint64) {
	c.bar += x
}
```

In this example, ContainerPlain has two `uint64` fields, `foo` and `bar`,
packed closely in memory.
When multiple goroutines access these fields concurrently,
false sharing may occur, causing performance degradation.

----

Example 2: `ContainerPadding` - cache line padding

```go
type ContainerPadding struct {
	foo uint64
	_   cpu.CacheLinePad
	bar uint64
}

func (c *ContainerPadding) AddFoo(x uint64) {
	c.foo += x
}

func (c *ContainerPadding) AddBar(x uint64) {
	c.bar += x
}
```

Here, we introduce padding between foo and bar using the `cpu.CacheLinePad` type.
This additional padding aligns foo and bar to different cache lines,
reducing the chances of false sharing and improving performance.

See: [pkg.go.dev/internal/cpu](https://pkg.go.dev/internal/cpu#CacheLinePad)

The type  `cpu.CacheLinePad` from `golang.org/x/sys` is implemented as:
```go
// CacheLinePad is used to pad structs to avoid false sharing.
type CacheLinePad struct{ _ [cacheLineSize]byte }
```

It's adding redundant memory of cache-line size to the  struct, hence these two variables
can not be put into the single cache-line and should be loaded separately.

---

Example 3: `ContainerArray` - using array instead of struct fields

```go
type ContainerArray [2]uint64

func (c *ContainerArray) AddFoo(x uint64) {
	c[0] += x
}

func (c *ContainerArray) AddBar(x uint64) {
	c[1] += x
}
```

Using an array, to see if it can address false sharing. But looking ahead, I can say that
it doesn't. In this case, the two fields are stored in a contiguous block of memory,
similar to a struct without padding, allowing false sharing to occur when accessed by multiple goroutines.

----


Example 4: `ContainerArrayPadding` - array with padding

```go
type ContainerArrayPadding [1 + 8 + 1]uint64

func (c *ContainerArrayPadding) AddFoo(x uint64) {
	c[0] += x
}

func (c *ContainerArrayPadding) AddBar(x uint64) {
	c[9] += x
}
```

Similar to the ContainerPadding example, we add padding to the array as redundant elements to ensure that the elements
of the array are not on the same cache line. This can lead to better performance compared to `ContainerArray`
when accessed by multiple goroutines.
Eight (8) in this implementation is common cache-line size: 64 bytes or 8 * uint64.

----


Now let's run benchmarks:
```go
func BenchmarkContainers(b *testing.B) {
	b.Run("plain", containerBenchmarker(new(ContainerPlain)))
	b.Run("padding", containerBenchmarker(new(ContainerPadding)))
	b.Run("array", containerBenchmarker(new(ContainerArray)))
	b.Run("array-padding", containerBenchmarker(new(ContainerArrayPadding)))
}
```

And see the result:
```txt
goos: linux
goarch: amd64
pkg: example.com
cpu: AMD Ryzen 7 5700U with Radeon Graphics
BenchmarkContainers/plain-16         	207490315	         5.897 ns/op
BenchmarkContainers/padding-16       	733552783	         1.629 ns/op
BenchmarkContainers/array-16         	209615830	         5.911 ns/op
BenchmarkContainers/array-padding-16 	729578326	         1.626 ns/op
PASS
ok  	example.com	6.360s
```

# Conclusion

In analyzing the benchmark results on an AMD Ryzen 7 5700U,
it's evident that the performance of each operation is already quite fast in Go.
However, the strategic use of padding in the `ContainerPadding` and `ContainerArrayPadding` examples showcases a noteworthy gain in performance,
achieving significantly lower operation times compared to their non-padded counterparts.

It's essential to note that in typical scenarios where structs are accessed frequently,
the optimization of cache lines through padding might offer a valuable performance boost.
While the default performance of Go is impressive, fine-tuning for specific use cases, such as high-frequency struct access,
can lead to measurable improvements.

In conclusion, while not necessary for every situation, optimizing for cache lines with padding becomes particularly relevant when dealing
with structs accessed multiple million times per second,
demonstrating that thoughtful struct design can indeed provide additional performance benefits in specific scenarios.

{{< share >}}
