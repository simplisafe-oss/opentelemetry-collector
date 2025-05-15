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
	timeBuckets TimeBuckets
	// startTimeBuckets TimeBuckets
	lastWriteTime time.Time
}

type TimeBuckets struct {
	bucketTiny          int64
	bucketOneSecond     int64
	bucketFiveSeconds   int64
	bucketTenSeconds    int64
	bucketTwentySeconds int64
	bucketThirtySeconds int64
	bucketOneMinute     int64
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

					logResourceAttrs := func(prefix string, metricName string, age time.Duration, resourceMetrics pmetric.ResourceMetrics) {
						header := fmt.Sprintf("%s: #%s %s", prefix, metricName, age)
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
								t.timeBuckets.bucketOneMinute++
								logResourceAttrs("metricAgeDebug", metricName, age, rm)
							} else if age > time.Second*30 {
								t.timeBuckets.bucketThirtySeconds++
								logResourceAttrs("metricAgeDebug", metricName, age, rm)
							} else if age > time.Second*20 {
								t.timeBuckets.bucketTwentySeconds++
							} else if age > time.Second*10 {
								t.timeBuckets.bucketTenSeconds++
							} else if age > time.Second*5 {
								t.timeBuckets.bucketFiveSeconds++
							} else if age > time.Second {
								t.timeBuckets.bucketOneSecond++
							} else {
								t.timeBuckets.bucketTiny++
							}

							// startAge := now.Sub(atFn(l).StartTimestamp().AsTime())

							// if startAge > time.Minute {
							// 	t.startTimeBuckets.bucketOneMinute++
							// 	logResourceAttrs("metricAgeStart", metricName, startAge, rm)
							// } else if startAge > time.Second*30 {
							// 	t.startTimeBuckets.bucketThirtySeconds++
							// 	logResourceAttrs("metricAgeStart", metricName, startAge, rm)
							// } else if startAge > time.Second*20 {
							// 	t.startTimeBuckets.bucketTwentySeconds++
							// } else if startAge > time.Second*10 {
							// 	t.startTimeBuckets.bucketTenSeconds++
							// } else if startAge > time.Second*5 {
							// 	t.startTimeBuckets.bucketFiveSeconds++
							// } else if startAge > time.Second {
							// 	t.startTimeBuckets.bucketOneSecond++
							// } else {
							// 	t.startTimeBuckets.bucketTiny++
							// }
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
			t.timeBuckets.bucketTiny,
			t.timeBuckets.bucketOneSecond,
			t.timeBuckets.bucketFiveSeconds,
			t.timeBuckets.bucketTenSeconds,
			t.timeBuckets.bucketTwentySeconds,
			t.timeBuckets.bucketThirtySeconds,
			t.timeBuckets.bucketOneMinute,
		))

		// Reset all buckets
		t.timeBuckets.bucketTiny = 0
		t.timeBuckets.bucketOneSecond = 0
		t.timeBuckets.bucketFiveSeconds = 0
		t.timeBuckets.bucketTenSeconds = 0
		t.timeBuckets.bucketTwentySeconds = 0
		t.timeBuckets.bucketThirtySeconds = 0
		t.timeBuckets.bucketOneMinute = 0

		// buf.buf.WriteString(fmt.Sprintf("MetricAgeStartBuckets: %d %d %d %d %d %d %d",
		// 	t.startTimeBuckets.bucketTiny,
		// 	t.startTimeBuckets.bucketOneSecond,
		// 	t.startTimeBuckets.bucketFiveSeconds,
		// 	t.startTimeBuckets.bucketTenSeconds,
		// 	t.startTimeBuckets.bucketTwentySeconds,
		// 	t.startTimeBuckets.bucketThirtySeconds,
		// 	t.startTimeBuckets.bucketOneMinute,
		// ))

		// // Reset all buckets
		// t.startTimeBuckets.bucketTiny = 0
		// t.startTimeBuckets.bucketOneSecond = 0
		// t.startTimeBuckets.bucketFiveSeconds = 0
		// t.startTimeBuckets.bucketTenSeconds = 0
		// t.startTimeBuckets.bucketTwentySeconds = 0
		// t.startTimeBuckets.bucketThirtySeconds = 0
		// t.startTimeBuckets.bucketOneMinute = 0

		t.lastWriteTime = now
	}

	return buf.buf.Bytes(), nil
}
