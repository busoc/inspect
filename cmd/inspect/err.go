package main

import (
  "fmt"
  "os"
)

const (
  EINVALID = 22
  EIO = 5
)

const (
  GenericErrCode = 5000 + iota
  TLEFormatErrCode
  BStarErrCode
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

func badUsage(n string) error {
  e := Error{
    Cause: fmt.Errorf(n),
    Code: EINVALID,
  }
  return &e
}
