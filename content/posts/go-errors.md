+++
draft = false
date = 2022-12-16T21:59:19+03:00
title = "The performance of Go error handling"
description = "Performance benchmarks for Go error handling common patterns and libraries"
slug = "go-errors-performance"
categories = ["go", "performance"]
tags = ["go"]
+++

In some languages, error or exception handling is a quite slow operation,
e.g. in [Java](https://stackoverflow.com/a/299315/1723695).
So the question about performance impact of Go error handling became an
interest for me.
It's obvious that creating, returning or printing errors does not differ from
creating, returning or printing any other interface type. But in Go we have
common patterns to wrap it (`Unwrap()` func)
and check or wrap underlying values (`Is(error) bool` and `As(any) bool`).
I created a set of benchmarks to test the performance of plain error value,
custom error structs, errors from `fmt.Errorf` and errors created by two
popular libraries [github.com/pkg/errors](https://github.com/pkg/errors)
and [go.uber.org/multierr](https://github.com/uber-go/multierr).


## What will be tested there

In these benchmarks I'm testing three cases:
 - Create and print error
 - Check error value using `errors.Is()`
 - Extract underlying error using `errors.As()`

*If you are not familiar with these methods, I'd recommending to read Go
[docs about errors](https://go.dev/blog/error-handling-and-go) first.*

Each benchmark tests performance and amount of memory allocations.

In each benchmark I'm using similar set of test targets:
 - Baseline - benchmark baseline, target with minimal impact on performance
 - Custom - custom struct for error
 - Wrap - custom struct to wrap underlying error, provide `Unwrap() error` method
 - `fmt.Errorf` - wrap error variable with `fmt.Errorf("%w", err)`
 - `errors.Wrap` - wrap error variable with stacktrace using `errors.Wrap(err)`
 func from `github.com/pkg/errors` lib.
 - `multierr` - append two error variables using `multierr.Append()` from
 `go.uber.org/multierr`.
 
All benchmark's code and target types are available here:
[/examples/go-errors](https://github.com/g4s8/g4s8.wtf/tree/master/examples/go-errors).

### Create and print

This test is quite simple - I just create a new error and call `Error()`
method.

For baseline one global error variable is used which was created using `errors.New()`.

Custom struct is a struct value with `string` field which prints it as `Error()`.

All wrapping types (custom wrap struct, `fmt.Errorf` and `errors.Wrap`) just wraps
global error variable (the same as used for baseline) and call `Error()` on it.

Multierr test combine two error variables into single error.

### errors.Is

This benchmark checks the performance of `errors.Is(err, target error) bool` func
with different `err` and `target` arguments.

Baseline - just check global variable error as `err` and `target`.

Custom - an error struct with `Is(error) bool` method:
```go
func (e *errIs) Is(target error) bool {
	if p, ok := target.(*errIs); ok {
		return e.x == p.x
	}
	return false
}
```

Wrap, `fmt.Error`, `errors.Wrap` and multierr are the same as in previous case.

### errors.As

Benchmark cases for `errors.As(err error, any target) bool` func.

Baseline - construct a new custom error struct and check the performance of
extracting it into same-type struct pointer.

Custom - use error struct with method `As`:
```go
func (e *errAs) As(target any) bool {
	if p, ok := target.(**errAs); ok {
		if *p == nil {
			*p = &errAs{x: e.x}
		} else {
			(*p).x = e.x
		}
		return true
	}
	return false
}
```

All wrappers are the same.

## Results

*The results of these benchmarks are computed on my local laptop, so
actual time values are not really representative, but it shows an order
and big differences between some benchmark targets.*

### Performance

Performance of all cases units is ns per operation.

| Test           | New + `Error()` | `errors.Is()` | `errors.As()` |
|----------------|-----------------|---------------|---------------|
| Baseline       | 1.505           | 5.678         | 61.64         |
| Custom         | 0.249           | 12.99         | 66.87         |
| Wrap           | 1.499           | 19.82         | 82.92         |
| `fmt.Errorf`   | 144.9           | 19.32         | 81.79         |
| `errors.Wrap`  | 909.3           | 33.11         | 117.7         |
| `multierr`     | 162.4           | 18.60         | 111.2         |


### Memory

Memory allocations units is allocations per operation.

| Test           | New + `Error()` | `errors.Is()` | `errors.As()` |
|----------------|-----------------|---------------|---------------|
| Baseline       | 0               | 0             | 0             |
| Custom         | 0               | 0             | 0             |
| Wrap           | 0               | 0             | 0             |
| `fmt.Errorf`   | 2               | 0             | 0             |
| `errors.Wrap`  | 5               | 0             | 0             |
| `multierr`     | 3               | 0             | 0             |

## Conclusion

Nothing out of the ordinary.
I'd expected these results when created benchmarks.
Go errors handling doesn't have performance drawbacks, most common operations takes just a number of
nanoseconds and no additional allocations. If you compare it with Java exceptions it'll be at least 100x faster.
The only case with similar to Java performance is `errors.Wrap` because it collects the stacktrace
data of method call, but it can provide more details.

But these results has a few interesting points:
 - The performance of custom `Is` and `As` just a little better than performance of struct with `Unwrap` method
 (13 vs 19 ns and 66 vs 82 ns), so it could be used for implementing custom logic based on error internals,
 not for performance benefits.
 - Create `fmt.Errorf` from base error is much faster than `errors.Wrap`. So use `errors.Wrap` if you need stacktrace data
 or if you don't care about additional ~1000ns on error creating. Custom wrapper struct could be even better than
 `fmt.Errorf`.
 - If you really care about performance when creating a new error, create value based struct error on stack - it may
 cost less than 1ns and zero allocations.
