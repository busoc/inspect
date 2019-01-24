package main

import (
	"fmt"
	"os"
	"syscall"

	"github.com/busoc/celest"
)

const (
	EINVALID = 22
	EIO      = 5
)

const (
	GenericErrCode = 5000 + iota
	TLEFormatErrCode
	TLEDataErrCode
	PropagationErrCode
	DragErrCode
)

type Error struct {
	Cause error
	Code  int
}

func (e *Error) Error() string {
	return e.Cause.Error()
}

func Exit(err error) {
	if err == nil {
		return
	}
	fmt.Fprintln(os.Stderr, err)

	code := EINVALID
	if e, ok := err.(*Error); ok {
		code = e.Code
	}
	os.Exit(code)
}

func fetchError(n string, c int) error {
	e := Error{
		Cause: fmt.Errorf("fail to fetch data from %s (%d)", n, c),
		Code:  EIO,
	}
	return &e
}

func badUsage(n string) error {
	e := Error{
		Cause: fmt.Errorf(n),
		Code:  EINVALID,
	}
	return &e
}

func checkError(err, parent error) error {
	if err == nil {
		return nil
	}
	switch err {
	case celest.ErrShortPeriod, celest.ErrBaseTime:
		return &Error{
			Cause: err,
			Code:  EINVALID,
		}
	default:
	}
	switch e := err.(type) {
	case *Error:
		return e
	case *celest.ParseError:
		return &Error{
			Cause: e,
			Code:  TLEDataErrCode,
		}
	case celest.PropagationError:
		return &Error{
			Cause: err,
			Code:  PropagationErrCode,
		}
	case celest.DragError:
		return &Error {
			Cause: err,
			Code: DragErrCode,
		}
	case celest.InvalidLenError, celest.MissingRowError:
		return &Error{
			Cause: err,
			Code:  TLEDataErrCode,
		}
	case *os.PathError:
		return checkError(e.Err, err)
	case syscall.Errno:
		if parent != nil {
			err = parent
		}
		return &Error{Cause: err, Code: int(e)}
	default:
		return err
	}
}
