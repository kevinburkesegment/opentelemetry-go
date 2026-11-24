// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tracetransform

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	"google.golang.org/protobuf/proto"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestSpansRetainsAfterNextExport(t *testing.T) {
	res := resource.NewSchemaless(attribute.String("service.name", "test"))
	scope := instrumentation.Scope{Name: "test"}

	s1 := tracetest.SpanStub{
		Name:       "s1",
		Attributes: []attribute.KeyValue{attribute.String("k", "v1")},
		Events: []tracesdk.Event{
			{Name: "e1", Attributes: []attribute.KeyValue{attribute.String("ek", "ev1")}},
		},
		Resource:             res,
		InstrumentationScope: scope,
	}.Snapshot()
	out1 := Spans([]tracesdk.ReadOnlySpan{s1})
	require.Len(t, out1, 1)
	require.Len(t, out1[0].ScopeSpans, 1)
	require.Len(t, out1[0].ScopeSpans[0].Spans, 1)
	firstAttrs := out1[0].ScopeSpans[0].Spans[0].Attributes
	require.Len(t, firstAttrs, 1)
	assert.Equal(t, "v1", firstAttrs[0].Value.GetStringValue())

	s2 := tracetest.SpanStub{
		Name:       "s2",
		Attributes: []attribute.KeyValue{attribute.String("k", "v2")},
		Events: []tracesdk.Event{
			{Name: "e1", Attributes: []attribute.KeyValue{attribute.String("ek", "ev2")}},
		},
		Resource:             res,
		InstrumentationScope: scope,
	}.Snapshot()
	out2 := Spans([]tracesdk.ReadOnlySpan{s2})
	require.Len(t, out2, 1)

	assert.Equal(t, "v1", out1[0].ScopeSpans[0].Spans[0].Attributes[0].Value.GetStringValue())
	assert.Equal(t, "ev1", out1[0].ScopeSpans[0].Spans[0].Events[0].Attributes[0].Value.GetStringValue())
	assert.Equal(t, "v2", out2[0].ScopeSpans[0].Spans[0].Attributes[0].Value.GetStringValue())
}

func TestSizedKeyValuesMatchesKeyValue(t *testing.T) {
	attrs := []attribute.KeyValue{
		attribute.Bool("bool", true),
		attribute.Int64("int", 1),
		attribute.Float64("float", 1.5),
		attribute.String("string", "v"),
		attribute.BoolSlice("bools", []bool{true, false}),
		attribute.Int64Slice("ints", []int64{1, 2}),
		attribute.Float64Slice("floats", []float64{1.5, 2.5}),
		attribute.StringSlice("strings", []string{"a", "b"}),
		attribute.ByteSlice("bytes", []byte("bytes")),
		attribute.Slice("slice",
			attribute.StringValue("a"),
			attribute.SliceValue(attribute.IntValue(1)),
			attribute.MapValue(attribute.String("k", "v")),
		),
		attribute.Map("map",
			attribute.String("k", "v"),
			attribute.Map("nested", attribute.Int("n", 1), attribute.Bool("b", true)),
		),
		{Key: "empty"},
		attribute.Slice("empty.slice"),
		attribute.Map("empty.map"),
	}

	want := make([]*commonpb.KeyValue, len(attrs))
	for i, kv := range attrs {
		want[i] = KeyValue(kv)
	}
	got := sizedKeyValues(attrs)
	require.Len(t, got, len(want))
	for i := range want {
		assert.Truef(t, proto.Equal(want[i], got[i]), "attribute %q:\nwant %v\ngot  %v", attrs[i].Key, want[i], got[i])
	}

	// KeyValues uses sizedKeyValues for lists longer than maxUnsizedLen.
	require.Greater(t, len(attrs), maxUnsizedLen)
	assert.Len(t, KeyValues(attrs), len(attrs))
}
