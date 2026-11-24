// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tracetransform

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
