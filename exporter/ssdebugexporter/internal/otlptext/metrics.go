// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otlptext // import "go.opentelemetry.io/collector/exporter/ssdebugexporter/internal/otlptext"

import (
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// NewTextMetricsMarshaler returns a pmetric.Marshaler to encode to OTLP text bytes.
func NewTextMetricsMarshaler() pmetric.Marshaler {
	return textMetricsMarshaler{}
}

type textMetricsMarshaler struct {
	bucketTiny          int64
	bucketOneSecond     int64
	bucketFiveSeconds   int64
	bucketTenSeconds    int64
	bucketTwentySeconds int64
	bucketThirtySeconds int64
	bucketOneMinute     int64
	lastWriteTime       time.Time
}

// MarshalMetrics pmetric.Metrics to OTLP text.
func (t textMetricsMarshaler) MarshalMetrics(md pmetric.Metrics) ([]byte, error) {
	now := time.Now()

	buf := dataBuffer{}
	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		// buf.logEntry("ResourceMetrics #%d", i)
		rm := rms.At(i)
		// buf.logEntry("Resource SchemaURL: %s", rm.SchemaUrl())
		// buf.logAttributes("Resource attributes", rm.Resource().Attributes())
		ilms := rm.ScopeMetrics()
		for j := 0; j < ilms.Len(); j++ {
			// buf.logEntry("ScopeMetrics #%d", j)
			ilm := ilms.At(j)
			// buf.logEntry("ScopeMetrics SchemaURL: %s", ilm.SchemaUrl())
			// buf.logInstrumentationScope(ilm.Scope())
			metrics := ilm.Metrics()
			for k := 0; k < metrics.Len(); k++ {

				// buf.logEntry("Metric #%d", k)
				metric := metrics.At(k)
				// buf.logMetricDescriptor(metric)
				// buf.logMetricDataPoints(metric)

				if !strings.HasPrefix(metric.Name(), "alb_") && !strings.HasPrefix(metric.Name(), "nlb_") {

					logResourceAttrs := func(metricName string, age time.Duration, resourceMetrics pmetric.ResourceMetrics) {
						header := fmt.Sprintf("metric age: #%s is %s old", metricName, age)
						buf.logAttributesOneLine(header, rm.Resource().Attributes())
					}

					// Helper function to process datapoints and log age if needed
					processDatapointAges := func(lenFn func() int, atFn func(int) interface {
						Timestamp() pcommon.Timestamp
						StartTimestamp() pcommon.Timestamp
					}, metricName string) {
						for l := 0; l < lenFn(); l++ {
							datapointTime := atFn(l).Timestamp().AsTime()
							age := now.Sub(datapointTime)

							if age > time.Minute {
								t.bucketOneMinute++
								logResourceAttrs(metricName, age, rm)
							} else if age > time.Second*30 {
								t.bucketThirtySeconds++
								logResourceAttrs(metricName, age, rm)
							} else if age > time.Second*20 {
								t.bucketTwentySeconds++
							} else if age > time.Second*10 {
								t.bucketTenSeconds++
							} else if age > time.Second*5 {
								t.bucketFiveSeconds++
							} else if age > time.Second {
								t.bucketOneSecond++
							} else {
								t.bucketTiny++
							}
						}
					}

					switch metric.Type() {
					case pmetric.MetricTypeGauge:
						dps := metric.Gauge().DataPoints()
						processDatapointAges(dps.Len, func(i int) interface {
							Timestamp() pcommon.Timestamp
							StartTimestamp() pcommon.Timestamp
						} {
							return dps.At(i)
						}, metric.Name())
					case pmetric.MetricTypeSum:
						dps := metric.Sum().DataPoints()
						processDatapointAges(dps.Len, func(i int) interface {
							Timestamp() pcommon.Timestamp
							StartTimestamp() pcommon.Timestamp
						} {
							return dps.At(i)
						}, metric.Name())
					case pmetric.MetricTypeHistogram:
						dps := metric.Histogram().DataPoints()
						processDatapointAges(dps.Len, func(i int) interface {
							Timestamp() pcommon.Timestamp
							StartTimestamp() pcommon.Timestamp
						} {
							return dps.At(i)
						}, metric.Name())
					case pmetric.MetricTypeExponentialHistogram:
						dps := metric.ExponentialHistogram().DataPoints()
						processDatapointAges(dps.Len, func(i int) interface {
							Timestamp() pcommon.Timestamp
							StartTimestamp() pcommon.Timestamp
						} {
							return dps.At(i)
						}, metric.Name())
					case pmetric.MetricTypeSummary:
						dps := metric.Summary().DataPoints()
						processDatapointAges(dps.Len, func(i int) interface {
							Timestamp() pcommon.Timestamp
							StartTimestamp() pcommon.Timestamp
						} {
							return dps.At(i)
						}, metric.Name())
					}
				}
			}
		}
	}

	// Check if 30 seconds have passed since last write
	if t.lastWriteTime.IsZero() || now.Sub(t.lastWriteTime) >= 30*time.Second {
		buf.buf.WriteString(fmt.Sprintf("MetricAgeBuckets: %d %d %d %d %d %d %d",
			t.bucketTiny,
			t.bucketOneSecond,
			t.bucketFiveSeconds,
			t.bucketTenSeconds,
			t.bucketTwentySeconds,
			t.bucketThirtySeconds,
			t.bucketOneMinute,
		))

		// Reset all buckets
		t.bucketTiny = 0
		t.bucketOneSecond = 0
		t.bucketFiveSeconds = 0
		t.bucketTenSeconds = 0
		t.bucketTwentySeconds = 0
		t.bucketThirtySeconds = 0
		t.bucketOneMinute = 0
		t.lastWriteTime = now
	}

	return buf.buf.Bytes(), nil
}
