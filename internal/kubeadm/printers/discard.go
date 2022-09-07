// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package printers

import (
	"io"

	"k8s.io/apimachinery/pkg/runtime"
)

type Discard struct{}

func (d Discard) PrintObj(obj runtime.Object, writer io.Writer) error {
	return nil
}

func (d Discard) Fprintf(writer io.Writer, format string, args ...interface{}) (n int, err error) {
	return
}

func (d Discard) Fprintln(writer io.Writer, args ...interface{}) (n int, err error) {
	return
}

func (d Discard) Printf(format string, args ...interface{}) (n int, err error) {
	return
}

func (d Discard) Println(args ...interface{}) (n int, err error) {
	return
}

func (d Discard) Flush(writer io.Writer, last bool) {
}

func (d Discard) Close(writer io.Writer) {
}
