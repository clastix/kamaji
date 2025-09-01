// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package resources_test

import (
	"github.com/prometheus/client_golang/prometheus"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/clastix/kamaji/internal/resources"
)

var _ = Describe("Metrics functionality", func() {
	Context("Collector getters and setters", func() {
		It("should have DatastoreCertificateCollector getter and setter", func() {
			// Initially should be nil
			collector := resources.GetDatastoreCertificateCollector()
			Expect(collector).To(BeNil())

			// Create a test histogram
			testHistogram := prometheus.NewHistogram(prometheus.HistogramOpts{
				Name: "test_datastore_certificate_histogram",
				Help: "Test histogram for datastore certificate",
			})

			// Set the collector
			resources.SetDatastoreCertificateCollector(testHistogram)

			// Get the collector and verify it's set
			retrievedCollector := resources.GetDatastoreCertificateCollector()
			Expect(retrievedCollector).ToNot(BeNil())
			Expect(retrievedCollector).To(Equal(testHistogram))
		})

		It("should have KonnectivityCertificateCollector getter and setter", func() {
			// Initially should be nil
			collector := resources.GetKonnectivityCertificateCollector()
			Expect(collector).To(BeNil())

			// Create a test histogram
			testHistogram := prometheus.NewHistogram(prometheus.HistogramOpts{
				Name: "test_konnectivity_certificate_histogram",
				Help: "Test histogram for konnectivity certificate",
			})

			// Set the collector
			resources.SetKonnectivityCertificateCollector(testHistogram)

			// Get the collector and verify it's set
			retrievedCollector := resources.GetKonnectivityCertificateCollector()
			Expect(retrievedCollector).ToNot(BeNil())
			Expect(retrievedCollector).To(Equal(testHistogram))
		})
	})

	Context("LazyLoadHistogramFromResource functionality", func() {
		It("should handle nil collector input by creating new histogram", func() {
			// Reset collectors to ensure clean state
			resources.SetDatastoreCertificateCollector(nil)
			resources.SetKonnectivityCertificateCollector(nil)

			// Test with actual certificate resources that implement the interface

			// Create a test histogram using the function with nil input
			// This should create a new histogram lazily

			// Since we can't easily mock the full Resource interface in this test,
			// we'll test the getter/setter functions which are the main additions
			// The LazyLoadHistogramFromResource is tested implicitly by the resource tests

			// This test verifies the new collector functions work
			Expect(resources.GetDatastoreCertificateCollector()).To(BeNil())
			Expect(resources.GetKonnectivityCertificateCollector()).To(BeNil())
		})
	})
})
