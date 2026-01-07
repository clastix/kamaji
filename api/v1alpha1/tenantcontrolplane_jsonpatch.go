// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

type JSONPatches []JSONPatch

type JSONPatch struct {
	// Op is the RFC 6902 JSON Patch operation.
	//+kubebuilder:validation:Enum=add;remove;replace;move;copy;test
	Op string `json:"op"`
	// Path specifies the target location in the JSON document. Use "/" to separate keys; "-" for appending to arrays.
	Path string `json:"path"`
	// From specifies the source location for move or copy operations.
	From string `json:"from,omitempty"`
	// Value is the operation value to be used when Op is add, replace, test.
	Value *apiextensionsv1.JSON `json:"value,omitempty"`
}

func (p JSONPatches) ToJSON() ([]byte, error) {
	if len(p) == 0 {
		return []byte("[]"), nil
	}

	buf := make([]byte, 0, 256)
	buf = append(buf, '[')

	for i, patch := range p {
		if i > 0 {
			buf = append(buf, ',')
		}

		buf = append(buf, '{')

		buf = append(buf, `"op":"`...)
		buf = appendEscapedString(buf, patch.Op)
		buf = append(buf, '"')

		buf = append(buf, `,"path":"`...)
		buf = appendEscapedString(buf, patch.Path)
		buf = append(buf, '"')

		if patch.From != "" {
			buf = append(buf, `,"from":"`...)
			buf = appendEscapedString(buf, patch.From)
			buf = append(buf, '"')
		}

		if patch.Value != nil {
			buf = append(buf, `,"value":`...)
			buf = append(buf, patch.Value.Raw...)
		}

		buf = append(buf, '}')
	}

	buf = append(buf, ']')

	return buf, nil
}

func appendEscapedString(dst []byte, s string) []byte {
	for i := range s {
		switch s[i] {
		case '\\', '"':
			dst = append(dst, '\\', s[i])
		case '\n':
			dst = append(dst, '\\', 'n')
		case '\r':
			dst = append(dst, '\\', 'r')
		case '\t':
			dst = append(dst, '\\', 't')
		default:
			dst = append(dst, s[i])
		}
	}

	return dst
}
