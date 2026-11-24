// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tracetransform

import (
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"

	"go.opentelemetry.io/otel/attribute"
)

// maxUnsizedLen is the longest attribute list converted one attribute at a
// time. For shorter lists, counting the values and allocating an array per
// value type costs more than the separate allocations it saves.
const maxUnsizedLen = 2

// attrCounts holds exact backing sizes for one attribute list, including
// nested SLICE/MAP contents. Sizing from actual contents avoids the sparse
// byte regressions of fixed per-span heuristics.
type attrCounts struct {
	kvs    int
	avs    int
	arrs   int
	arrW   int
	kvls   int
	kvW    int
	strs   int
	bools  int
	ints   int
	floats int
	bytes  int
}

// listStorage provides per-export backing arrays for one attribute list.
// Each export owns its storage, nothing is reset or reused, so Go keeps it
// alive for as long as a client retains the tree.
type listStorage struct {
	kvs    []commonpb.KeyValue
	avs    []commonpb.AnyValue
	arrs   []commonpb.ArrayValue
	arrW   []commonpb.AnyValue_ArrayValue
	kvls   []commonpb.KeyValueList
	kvW    []commonpb.AnyValue_KvlistValue
	strs   []commonpb.AnyValue_StringValue
	bools  []commonpb.AnyValue_BoolValue
	ints   []commonpb.AnyValue_IntValue
	floats []commonpb.AnyValue_DoubleValue
	bytes  []commonpb.AnyValue_BytesValue

	kvsIdx    int
	avsIdx    int
	arrsIdx   int
	arrWIdx   int
	kvlsIdx   int
	kvWIdx    int
	strsIdx   int
	boolsIdx  int
	intsIdx   int
	floatsIdx int
	bytesIdx  int
}

// init allocates the backing arrays for c. Arrays for value types the list
// does not contain are left nil, avoiding a runtime call for each.
func (s *listStorage) init(c attrCounts) {
	if c.kvs > 0 {
		s.kvs = make([]commonpb.KeyValue, c.kvs)
	}
	if c.avs > 0 {
		s.avs = make([]commonpb.AnyValue, c.avs)
	}
	if c.arrs > 0 {
		s.arrs = make([]commonpb.ArrayValue, c.arrs)
	}
	if c.arrW > 0 {
		s.arrW = make([]commonpb.AnyValue_ArrayValue, c.arrW)
	}
	if c.kvls > 0 {
		s.kvls = make([]commonpb.KeyValueList, c.kvls)
	}
	if c.kvW > 0 {
		s.kvW = make([]commonpb.AnyValue_KvlistValue, c.kvW)
	}
	if c.strs > 0 {
		s.strs = make([]commonpb.AnyValue_StringValue, c.strs)
	}
	if c.bools > 0 {
		s.bools = make([]commonpb.AnyValue_BoolValue, c.bools)
	}
	if c.ints > 0 {
		s.ints = make([]commonpb.AnyValue_IntValue, c.ints)
	}
	if c.floats > 0 {
		s.floats = make([]commonpb.AnyValue_DoubleValue, c.floats)
	}
	if c.bytes > 0 {
		s.bytes = make([]commonpb.AnyValue_BytesValue, c.bytes)
	}
}

func (s *listStorage) allocKV() *commonpb.KeyValue {
	v := &s.kvs[s.kvsIdx]
	s.kvsIdx++
	return v
}

func (s *listStorage) allocAV() *commonpb.AnyValue {
	v := &s.avs[s.avsIdx]
	s.avsIdx++
	return v
}

func countValue(v attribute.Value, c *attrCounts) {
	c.avs++
	switch v.Type() {
	case attribute.BOOL:
		c.bools++
	case attribute.INT64:
		c.ints++
	case attribute.FLOAT64:
		c.floats++
	case attribute.STRING:
		c.strs++
	case attribute.BOOLSLICE:
		n := len(v.AsBoolSlice())
		c.avs += n
		c.bools += n
		c.arrs++
		c.arrW++
	case attribute.INT64SLICE:
		n := len(v.AsInt64Slice())
		c.avs += n
		c.ints += n
		c.arrs++
		c.arrW++
	case attribute.FLOAT64SLICE:
		n := len(v.AsFloat64Slice())
		c.avs += n
		c.floats += n
		c.arrs++
		c.arrW++
	case attribute.STRINGSLICE:
		n := len(v.AsStringSlice())
		c.avs += n
		c.strs += n
		c.arrs++
		c.arrW++
	case attribute.BYTESLICE:
		c.bytes++
	case attribute.SLICE:
		c.arrs++
		c.arrW++
		for _, e := range v.AsSlice() {
			countValue(e, c)
		}
	case attribute.MAP:
		c.kvls++
		c.kvW++
		m := v.AsMap()
		c.kvs += len(m)
		for _, kv := range m {
			countValue(kv.Value, c)
		}
	case attribute.EMPTY:
	default:
		c.strs++
	}
}

func countAttrs(attrs []attribute.KeyValue) attrCounts {
	var c attrCounts
	c.kvs += len(attrs)
	for _, kv := range attrs {
		countValue(kv.Value, &c)
	}
	return c
}

func sizedKeyValues(attrs []attribute.KeyValue) []*commonpb.KeyValue {
	if len(attrs) == 0 {
		return nil
	}
	var s listStorage
	s.init(countAttrs(attrs))
	out := make([]*commonpb.KeyValue, 0, len(attrs))
	for _, kv := range attrs {
		pbKV := s.allocKV()
		pbKV.Key = string(kv.Key)
		pbKV.Value = s.value(kv.Value)
		out = append(out, pbKV)
	}
	return out
}

func (s *listStorage) value(v attribute.Value) *commonpb.AnyValue {
	av := s.allocAV()
	switch v.Type() {
	case attribute.BOOL:
		w := &s.bools[s.boolsIdx]
		s.boolsIdx++
		w.BoolValue = v.AsBool()
		av.Value = w
	case attribute.INT64:
		w := &s.ints[s.intsIdx]
		s.intsIdx++
		w.IntValue = v.AsInt64()
		av.Value = w
	case attribute.FLOAT64:
		w := &s.floats[s.floatsIdx]
		s.floatsIdx++
		w.DoubleValue = v.AsFloat64()
		av.Value = w
	case attribute.STRING:
		w := &s.strs[s.strsIdx]
		s.strsIdx++
		w.StringValue = v.AsString()
		av.Value = w
	case attribute.BOOLSLICE:
		vals := v.AsBoolSlice()
		converted := make([]*commonpb.AnyValue, len(vals))
		for i, e := range vals {
			eav := s.allocAV()
			w := &s.bools[s.boolsIdx]
			s.boolsIdx++
			w.BoolValue = e
			eav.Value = w
			converted[i] = eav
		}
		arr := &s.arrs[s.arrsIdx]
		s.arrsIdx++
		arr.Values = converted
		w := &s.arrW[s.arrWIdx]
		s.arrWIdx++
		w.ArrayValue = arr
		av.Value = w
	case attribute.INT64SLICE:
		vals := v.AsInt64Slice()
		converted := make([]*commonpb.AnyValue, len(vals))
		for i, e := range vals {
			eav := s.allocAV()
			w := &s.ints[s.intsIdx]
			s.intsIdx++
			w.IntValue = e
			eav.Value = w
			converted[i] = eav
		}
		arr := &s.arrs[s.arrsIdx]
		s.arrsIdx++
		arr.Values = converted
		w := &s.arrW[s.arrWIdx]
		s.arrWIdx++
		w.ArrayValue = arr
		av.Value = w
	case attribute.FLOAT64SLICE:
		vals := v.AsFloat64Slice()
		converted := make([]*commonpb.AnyValue, len(vals))
		for i, e := range vals {
			eav := s.allocAV()
			w := &s.floats[s.floatsIdx]
			s.floatsIdx++
			w.DoubleValue = e
			eav.Value = w
			converted[i] = eav
		}
		arr := &s.arrs[s.arrsIdx]
		s.arrsIdx++
		arr.Values = converted
		w := &s.arrW[s.arrWIdx]
		s.arrWIdx++
		w.ArrayValue = arr
		av.Value = w
	case attribute.STRINGSLICE:
		vals := v.AsStringSlice()
		converted := make([]*commonpb.AnyValue, len(vals))
		for i, e := range vals {
			eav := s.allocAV()
			w := &s.strs[s.strsIdx]
			s.strsIdx++
			w.StringValue = e
			eav.Value = w
			converted[i] = eav
		}
		arr := &s.arrs[s.arrsIdx]
		s.arrsIdx++
		arr.Values = converted
		w := &s.arrW[s.arrWIdx]
		s.arrWIdx++
		w.ArrayValue = arr
		av.Value = w
	case attribute.BYTESLICE:
		w := &s.bytes[s.bytesIdx]
		s.bytesIdx++
		w.BytesValue = v.AsByteSlice()
		av.Value = w
	case attribute.SLICE:
		vals := v.AsSlice()
		converted := make([]*commonpb.AnyValue, len(vals))
		for i, e := range vals {
			converted[i] = s.value(e)
		}
		arr := &s.arrs[s.arrsIdx]
		s.arrsIdx++
		arr.Values = converted
		w := &s.arrW[s.arrWIdx]
		s.arrWIdx++
		w.ArrayValue = arr
		av.Value = w
	case attribute.MAP:
		m := v.AsMap()
		entries := make([]*commonpb.KeyValue, 0, len(m))
		for _, kv := range m {
			pbKV := s.allocKV()
			pbKV.Key = string(kv.Key)
			pbKV.Value = s.value(kv.Value)
			entries = append(entries, pbKV)
		}
		kvl := &s.kvls[s.kvlsIdx]
		s.kvlsIdx++
		kvl.Values = entries
		w := &s.kvW[s.kvWIdx]
		s.kvWIdx++
		w.KvlistValue = kvl
		av.Value = w
	case attribute.EMPTY:
	default:
		w := &s.strs[s.strsIdx]
		s.strsIdx++
		w.StringValue = "INVALID"
		av.Value = w
	}
	return av
}
