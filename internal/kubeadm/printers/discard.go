// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package printers

import (
	"io"

	"k8s.io/apimachinery/pkg/runtime"
)

type Discard struct{}

func (d Discard) PrintObj(runtime.Object, io.Writer) error {
	return nil
}

func (d Discard) Fprintf(io.Writer, string, ...interface{}) (n int, err error) {
	return
}

func (d Discard) Fprintln(io.Writer, ...interface{}) (n int, err error) {
	return
}

func (d Discard) Printf(string, ...interface{}) (n int, err error) {
	return
}

func (d Discard) Println(...interface{}) (n int, err error) {
	return
}

func (d Discard) Flush(io.Writer, bool) {
}

func (d Discard) Close(io.Writer) {
}
