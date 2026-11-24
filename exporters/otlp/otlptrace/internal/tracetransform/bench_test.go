// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tracetransform

import (
	"encoding/binary"
	"runtime"
	"testing"
	"time"

	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

// BenchmarkSpansAttributes transforms batches of spans with few and many
// attributes. The retained-B/export metric is the heap kept alive by one
// transformed batch while it is retained, as by a client or gRPC interceptor
// that holds the request after UploadTraces returns.
func BenchmarkSpansAttributes(b *testing.B) {
	for _, bc := range []struct {
		name  string
		spans []tracesdk.ReadOnlySpan
	}{
		{"Sparse", sparseSpans(100)},
		{"Dense", denseSpans(100)},
	} {
		b.Run(bc.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_ = Spans(bc.spans)
			}
			b.ReportMetric(retainedBytes(bc.spans), "retained-B/export")
		})
	}
}

// retainedBytes returns the average heap kept alive by each of several
// retained transformations of spans.
func retainedBytes(spans []tracesdk.ReadOnlySpan) float64 {
	const n = 64
	out := make([][]*tracepb.ResourceSpans, n)

	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	for i := range out {
		out[i] = Spans(spans)
	}
	runtime.GC()
	runtime.ReadMemStats(&after)
	runtime.KeepAlive(out)

	return float64(int64(after.HeapAlloc)-int64(before.HeapAlloc)) / n
}

// sparseSpans returns n read-only spans with few attributes: one resource,
// scope, event, and link attribute each, and two span attributes.
func sparseSpans(n int) []tracesdk.ReadOnlySpan {
	res := resource.NewSchemaless(attribute.String("service.name", "sparse"))
	scope := instrumentation.Scope{
		Name:       "sparse",
		Version:    "v0.1.0",
		Attributes: attribute.NewSet(attribute.String("scope.attr", "v")),
	}
	start := time.Date(2020, time.December, 8, 20, 23, 0, 0, time.UTC)

	stubs := make(tracetest.SpanStubs, n)
	for i := range stubs {
		stubs[i] = tracetest.SpanStub{
			SpanContext: benchSpanContext(i, 0),
			Name:        "sparse",
			StartTime:   start,
			EndTime:     start.Add(time.Millisecond),
			Attributes: []attribute.KeyValue{
				attribute.String("http.request.method", "GET"),
				attribute.Int("http.response.status_code", 200),
			},
			Events: []tracesdk.Event{
				{
					Name:       "event",
					Time:       start,
					Attributes: []attribute.KeyValue{attribute.String("event.attr", "v")},
				},
			},
			Links: []tracesdk.Link{
				{
					SpanContext: benchSpanContext(i, 1),
					Attributes:  []attribute.KeyValue{attribute.String("link.attr", "v")},
				},
			},
			Resource:             res,
			InstrumentationScope: scope,
		}
	}
	return stubs.Snapshots()
}

// denseSpans returns n read-only spans with many attributes, including slice
// and map values: 6 resource attributes, 3 scope attributes, 16 span
// attributes, 4 events and 2 links with 4 attributes each.
func denseSpans(n int) []tracesdk.ReadOnlySpan {
	res := resource.NewSchemaless(
		attribute.String("service.name", "dense"),
		attribute.String("service.version", "v1.2.3"),
		attribute.String("service.instance.id", "6a1b2c3d"),
		attribute.String("host.name", "host-1"),
		attribute.String("cloud.region", "us-east-1"),
		attribute.StringSlice("process.command_args", []string{"server", "-v"}),
	)
	scope := instrumentation.Scope{
		Name:    "dense",
		Version: "v0.1.0",
		Attributes: attribute.NewSet(
			attribute.String("scope.a", "v"),
			attribute.Int("scope.b", 1),
			attribute.Bool("scope.c", true),
		),
	}
	start := time.Date(2020, time.December, 8, 20, 23, 0, 0, time.UTC)

	attrs := func(prefix string) []attribute.KeyValue {
		return []attribute.KeyValue{
			attribute.String(prefix+".string", "value"),
			attribute.Int(prefix+".int", 42),
			attribute.Float64(prefix+".float", 1.5),
			attribute.Bool(prefix+".bool", true),
		}
	}

	stubs := make(tracetest.SpanStubs, n)
	for i := range stubs {
		events := make([]tracesdk.Event, 4)
		for j := range events {
			events[j] = tracesdk.Event{Name: "event", Time: start, Attributes: attrs("event")}
		}
		stubs[i] = tracetest.SpanStub{
			SpanContext: benchSpanContext(i, 0),
			Parent:      benchSpanContext(i, 1),
			Name:        "dense",
			StartTime:   start,
			EndTime:     start.Add(time.Millisecond),
			Attributes: []attribute.KeyValue{
				attribute.String("http.request.method", "POST"),
				attribute.String("http.route", "/api/v1/items/{id}"),
				attribute.Int("http.response.status_code", 201),
				attribute.String("url.full", "https://example.com/api/v1/items/1"),
				attribute.String("server.address", "example.com"),
				attribute.Int("server.port", 443),
				attribute.String("network.protocol.version", "1.1"),
				attribute.String("user_agent.original", "bench/1.0"),
				attribute.Int64("http.request.body.size", 1024),
				attribute.Int64("http.response.body.size", 2048),
				attribute.Float64("app.duration", 12.5),
				attribute.Bool("app.cached", false),
				attribute.StringSlice("app.tags", []string{"a", "b", "c"}),
				attribute.Int64Slice("app.ids", []int64{1, 2, 3}),
				attribute.Map("app.user",
					attribute.String("id", "u-123"),
					attribute.String("tier", "gold"),
				),
				attribute.String("app.request_id", "req-1"),
			},
			Events: events,
			Links: []tracesdk.Link{
				{SpanContext: benchSpanContext(i, 2), Attributes: attrs("link")},
				{SpanContext: benchSpanContext(i, 3), Attributes: attrs("link")},
			},
			Resource:             res,
			InstrumentationScope: scope,
		}
	}
	return stubs.Snapshots()
}

func benchSpanContext(i, j int) trace.SpanContext {
	// Distinct, valid IDs for span i and its related span j.
	tid := trace.TraceID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	binary.BigEndian.PutUint64(tid[8:], uint64(max(i, 0)))
	sid := trace.SpanID{1, 2, 3, 4, 5, 6, 7, 8}
	binary.BigEndian.PutUint32(sid[:4], uint32(max(i, 0)&0x7fffffff))
	binary.BigEndian.PutUint32(sid[4:], uint32(max(j, 0)&0x7fffffff)+1)
	return trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    tid,
		SpanID:     sid,
		TraceFlags: trace.FlagsSampled,
	})
}
