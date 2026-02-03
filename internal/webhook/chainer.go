// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"gomodules.xyz/jsonpatch/v2"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/clastix/kamaji/internal/webhook/handlers"
)

type handlersChainer struct {
	decoder admission.Decoder
}

//nolint:gocognit
func (h handlersChainer) Handler(object runtime.Object, routeHandlers ...handlers.Handler) admission.HandlerFunc {
	return func(ctx context.Context, req admission.Request) admission.Response {
		var decodedObj, oldDecodedObj runtime.Object
		if object != nil {
			decodedObj, oldDecodedObj = object.DeepCopyObject(), object.DeepCopyObject()

			switch req.Operation {
			case admissionv1.Delete:
				// When deleting the OldObject struct field contains the object being deleted:
				// https://github.com/kubernetes/kubernetes/pull/76346
				if err := h.decoder.DecodeRaw(req.OldObject, decodedObj); err != nil {
					return admission.Errored(http.StatusInternalServerError, fmt.Errorf("unable to decode deleted object into %T: %w", object, err))
				}
			default:
				if err := h.decoder.Decode(req, decodedObj); err != nil {
					return admission.Errored(http.StatusInternalServerError, fmt.Errorf("unable to decode into %T: %w", object, err))
				}
			}
		}

		fnInvoker := func(fn func(runtime.Object) handlers.AdmissionResponse) (patches []jsonpatch.JsonPatchOperation, err error) {
			patch, err := fn(decodedObj)(ctx, req)
			if err != nil {
				return nil, err
			}

			if patch != nil {
				patches = append(patches, patch...)
			}

			return patches, nil
		}

		var patches []jsonpatch.JsonPatchOperation

		switch req.Operation {
		case admissionv1.Create:
			for _, routeHandler := range routeHandlers {
				handlerPatches, err := fnInvoker(routeHandler.OnCreate)
				if err != nil {
					return admission.Denied(err.Error())
				}

				patches = append(patches, handlerPatches...)
			}
		case admissionv1.Update:
			if err := h.decoder.DecodeRaw(req.OldObject, oldDecodedObj); err != nil {
				return admission.Errored(http.StatusInternalServerError, fmt.Errorf("unable to decode old object into %T: %w", object, err))
			}

			for _, routeHandler := range routeHandlers {
				handlerPatches, err := routeHandler.OnUpdate(decodedObj, oldDecodedObj)(ctx, req)
				if err != nil {
					return admission.Denied(err.Error())
				}

				patches = append(patches, handlerPatches...)
			}
		case admissionv1.Delete:
			for _, routeHandler := range routeHandlers {
				handlerPatches, err := fnInvoker(routeHandler.OnDelete)
				if err != nil {
					return admission.Denied(err.Error())
				}

				patches = append(patches, handlerPatches...)
			}
		case admissionv1.Connect:
			break
		}

		if len(patches) > 0 {
			return admission.Patched("patching required", patches...)
		}

		return admission.Allowed(fmt.Sprintf("%s operation allowed", strings.ToLower(string(req.Operation))))
	}
}
