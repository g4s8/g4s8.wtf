package main

import "strconv"

type errWrap struct {
	err error
}

func (e *errWrap) Error() string {
	return e.err.Error()
}

func (e *errWrap) Unwrap() error {
	return e.err
}

type errIs struct {
	x int
}

func (e *errIs) Error() string {
	return strconv.Itoa(e.x)
}

func (e *errIs) Is(target error) bool {
	if p, ok := target.(*errIs); ok {
		return e.x == p.x
	}
	return false
}

type errAs struct {
	x int
}

func (e *errAs) Error() string {
	return strconv.Itoa(e.x)
}

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

type errCustom struct {
	msg string
}

func (e errCustom) Error() string {
	return e.msg
}
