package main

import (
	"errors"
	"fmt"
	"testing"

	pkgerr "github.com/pkg/errors"
	"go.uber.org/multierr"
)

var (
	errTest  = errors.New("test error")
	errTest2 = errors.New("test error")
)

// benchmarks

func BenchmarkError(b *testing.B) {
	b.Run("baseline", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = errTest.Error()
		}
	})
	b.Run("custom-err", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			err := errCustom{"test"}
			_ = err.Error()
		}
	})
	b.Run("wrap-struct", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			err := &errWrap{err: errTest}
			_ = err.Error()
		}
	})
	b.Run("wrap-fmt", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			err := fmt.Errorf("%w", errTest)
			_ = err.Error()
		}
	})
	b.Run("pkg-errors", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			err := pkgerr.Wrap(errTest, "test")
			_ = err.Error()
		}
	})
	b.Run("multierr", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			err := multierr.Append(errTest, errTest2)
			_ = err.Error()
		}
	})
}

func BenchmarkIs(b *testing.B) {
	b.Run("baseline", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			t := errors.Is(errTest, errTest)
			if !t {
				b.Fail()
			}
		}
	})
	b.Run("custom-is", func(b *testing.B) {
		is := errIs{4}
		target := &errIs{4}
		for i := 0; i < b.N; i++ {
			t := errors.Is(target, &is)
			if !t {
				b.Fail()
			}
		}
	})
	b.Run("wrap-struct", func(b *testing.B) {
		target := &errWrap{err: errTest}
		for i := 0; i < b.N; i++ {
			t := errors.Is(target, errTest)
			if !t {
				b.Fail()
			}
		}
	})
	b.Run("wrap-fmt", func(b *testing.B) {
		target := fmt.Errorf("%w", errTest)
		for i := 0; i < b.N; i++ {
			t := errors.Is(target, errTest)

			if !t {
				b.Fail()
			}

		}
	})
	b.Run("pkg-errors", func(b *testing.B) {
		target := pkgerr.Wrap(errTest, "test")
		for i := 0; i < b.N; i++ {
			t := errors.Is(target, errTest)
			if !t {
				b.Fail()
			}
		}
	})
	b.Run("multierr", func(b *testing.B) {
		target := multierr.Append(errTest, errTest2)
		for i := 0; i < b.N; i++ {
			t := errors.Is(target, errTest)
			if !t {
				b.Fail()
			}
		}
	})
}

func BenchmarkAs(b *testing.B) {
	b.Run("baseline", func(b *testing.B) {
		var as errCustom
		target := errCustom{""}
		for i := 0; i < b.N; i++ {
			t := errors.As(target, &as)
			if !t {
				b.Fail()
			}
		}
	})
	b.Run("custom-as", func(b *testing.B) {
		var as *errAs
		target := &errAs{42}
		for i := 0; i < b.N; i++ {
			t := errors.As(target, &as)
			if !t {
				b.Fail()
			}
		}
	})
	b.Run("wrap-struct", func(b *testing.B) {
		var as errCustom
		target := &errWrap{err: errCustom{"1"}}
		for i := 0; i < b.N; i++ {
			t := errors.As(target, &as)
			if !t {
				b.Fail()
			}
		}
	})
	b.Run("wrap-fmt", func(b *testing.B) {
		var as errCustom
		target := fmt.Errorf("%w", errCustom{""})
		for i := 0; i < b.N; i++ {
			t := errors.As(target, &as)
			if !t {
				b.Fail()
			}
		}
	})
	b.Run("pkg-errors", func(b *testing.B) {
		var as errCustom
		target := pkgerr.Wrap(errCustom{""}, "test")
		for i := 0; i < b.N; i++ {
			t := errors.As(target, &as)
			if !t {
				b.Fail()
			}
		}
	})
	b.Run("multierr", func(b *testing.B) {
		var as errCustom
		target := multierr.Append(errCustom{""}, errCustom{""})
		for i := 0; i < b.N; i++ {
			t := errors.As(target, &as)
			if !t {
				b.Fail()
			}
		}
	})
}
