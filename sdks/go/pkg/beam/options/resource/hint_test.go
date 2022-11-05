// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package resource

import (
	"bytes"
	"fmt"
	"math"
	"reflect"
	"testing"

	pipepb "github.com/apache/beam/sdks/v2/go/pkg/beam/model/pipeline_v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func TestAcceleratorHint_MergeWith(t *testing.T) {
	inner := acceleratorHint{value: "inner"}
	outer := acceleratorHint{value: "outer"}
	if got, want := inner.MergeWith(outer), inner; got != want {
		t.Errorf("%v.MergeWith(%v) = %v, want %v", inner, outer, got, want)
	}
}

func TestAcceleratorHint_Payload(t *testing.T) {
	want := []byte("want")
	h := acceleratorHint{value: "want"}
	if got := h.Payload(); !bytes.Equal(got, want) {
		t.Errorf("%v.Payload() = %v, want %v", h, got, want)
	}
}

func TestMinRamBytesHint_MergeWith(t *testing.T) {
	low := minRamHint{value: 2}
	high := minRamHint{value: 12e7}

	if got, want := low.MergeWith(high), high; got != want {
		t.Errorf("%v.MergeWith(%v) = %v, want %v", low, high, got, want)
	}
	if got, want := high.MergeWith(low), high; got != want {
		t.Errorf("%v.MergeWith(%v) = %v, want %v", high, low, got, want)
	}
}

func TestMinRamBytesHint_Payload(t *testing.T) {
	tests := []struct {
		value   int64
		payload string
	}{
		{math.MinInt64, "-9223372036854775808"},
		{-1, "-1"},
		{0, "0"},
		{2, "2"},
		{11, "11"},
		{2003, "2003"},
		{1.2e7, "12000000"},
		{math.MaxInt64, "9223372036854775807"},
	}

	for _, test := range tests {
		h := minRamHint{value: test.value}
		if got, want := h.Payload(), []byte(test.payload); !bytes.Equal(got, want) {
			t.Errorf("%v.Payload() = %v, want %v", h, got, want)
		}
	}
}

func TestParseMinRamHint(t *testing.T) {
	tests := []struct {
		value   string
		payload string
	}{
		{"0", "0"},
		{"2", "2"},
		{"11", "11"},
		{"2003", "2003"},
		{"1.23MB", "1230000"},
		{"1.23MiB", "1289748"},
		{"4GB", "4000000000"},
		{"2GiB", "2147483648"},
		{"1.4KiB", "1433"},
	}

	for _, test := range tests {
		h := ParseMinRam(test.value)
		if got, want := h.Payload(), []byte(test.payload); !bytes.Equal(got, want) {
			t.Errorf("%v.Payload() = %v, want %v", h, string(got), string(want))
		}
	}
}

func TestParseMinRamHint_panic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("want ParseMinRam to panic")
		}
	}()
	ParseMinRam("a bad byte string")
}

// We copy the URN from the proto for use as a constant rather than perform a direct look up
// each time, or increase initialization time. However we do need to validate that they are
// correct, and match the standard hint urns, so that's done here.
func TestStandardHintUrns(t *testing.T) {
	var props = (pipepb.StandardResourceHints_Enum)(0).Descriptor().Values()

	getStandardURN := func(e pipepb.StandardResourceHints_Enum) string {
		return proto.GetExtension(props.ByNumber(protoreflect.EnumNumber(e)).Options(), pipepb.E_BeamUrn).(string)
	}

	tests := []struct {
		h   Hint
		urn string
	}{{
		h:   Accelerator("type:foo;count:bar;optional_option"),
		urn: getStandardURN(pipepb.StandardResourceHints_ACCELERATOR),
	}, {
		h:   MinRamBytes(2e9),
		urn: getStandardURN(pipepb.StandardResourceHints_MIN_RAM_BYTES),
	}}
	for _, test := range tests {
		if got, want := test.h.URN(), test.urn; got != want {
			t.Errorf("Checked urn for %T, got %q, want %q", test.h, got, want)
		}
	}
}

type customHint struct {
}

func (customHint) URN() string {
	return "top:secret:custom:urn"
}

func (customHint) Payload() []byte {
	return []byte("custom")
}

func (h customHint) MergeWith(outer Hint) Hint {
	return h
}

func TestHints_Equal(t *testing.T) {
	hs := NewHints(MinRamBytes(2e9), Accelerator("type:pants;count1;install-pajamas"))

	if got, want := hs.Equal(hs), true; got != want {
		t.Errorf("Self equal test: hs.Equal(hs) = %v, want %v", got, want)
	}
	eq := NewHints(MinRamBytes(2e9), Accelerator("type:pants;count1;install-pajamas"))
	if got, want := hs.Equal(eq), true; got != want {
		t.Errorf("identical equal test: hs.Equal(eq) = %v, want %v", got, want)
	}
	neqLenShort := NewHints(MinRamBytes(2e9))
	if got, want := hs.Equal(neqLenShort), false; got != want {
		t.Errorf("too short equal test: hs.Equal(neqLenShort) = %v, want %v", got, want)
	}
	ch := customHint{}
	neqLenLong := NewHints(MinRamBytes(2e9), Accelerator("type:pants;count1;install-pajamas"), ch)
	if got, want := hs.Equal(neqLenLong), false; got != want {
		t.Errorf("too long equal test: hs.Equal(neqLenLong) = %v, want %v", got, want)
	}
	neqSameHintTypes := NewHints(MinRamBytes(2e10), Accelerator("type:pants;count1;install-pajamas"))
	if got, want := hs.Equal(neqSameHintTypes), false; got != want {
		t.Errorf("sameHintTypes equal test: hs.Equal(neqLenSameHintTypes) = %v, want %v", got, want)
	}
	neqSameHintTypes2 := NewHints(MinRamBytes(2e9), Accelerator("type:pants;count1;install-pajama"))
	if got, want := hs.Equal(neqSameHintTypes2), false; got != want {
		t.Errorf("sameHintTypes2 equal test: hs.Equal(neqLenSameHintTypes2) = %v, want %v", got, want)
	}
	neqDiffHintTypes2 := NewHints(MinRamBytes(2e9), ch)
	if got, want := hs.Equal(neqDiffHintTypes2), false; got != want {
		t.Errorf("diffHintTypes equal test: hs.Equal(neqDiffHintTypes2) = %v, want %v", got, want)
	}
}

func TestHints_MergeWithOuter(t *testing.T) {

	lowRam, medRam, highRam := MinRamBytes(2e7), MinRamBytes(2e9), MinRamBytes(2e10)

	pantsAcc := Accelerator("type:pants;count1;install-pajamas")
	jeansAcc := Accelerator("type:jeans;count1;")
	custom := customHint{}

	hsA := NewHints(medRam, pantsAcc)
	hsB := NewHints(highRam, jeansAcc)
	hsC := NewHints(lowRam, custom)

	tests := []struct {
		inner, outer, want Hints
	}{
		{hsA, hsA, hsA},
		{hsB, hsB, hsB},
		{hsC, hsC, hsC},
		{hsA, hsB, NewHints(highRam, pantsAcc)},
		{hsB, hsA, NewHints(highRam, jeansAcc)},
		{hsA, hsC, NewHints(medRam, pantsAcc, custom)},
		{hsC, hsA, NewHints(medRam, pantsAcc, custom)},
		{hsB, hsC, NewHints(highRam, jeansAcc, custom)},
		{hsC, hsB, NewHints(highRam, jeansAcc, custom)},
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			if got, want := test.inner.MergeWithOuter(test.outer), test.want; !got.Equal(want) {
				t.Errorf("%v.MergeWithOuter(%v) = %v, want %v", test.inner, test.outer, got, want)
			}
		})
	}
}

func TestHints_Payloads(t *testing.T) {
	hs := NewHints(MinRamBytes(2e9), Accelerator("type:jeans;count1;"))

	got := hs.Payloads()
	want := map[string][]byte{
		"beam:resources:min_ram_bytes:v1": []byte("2000000000"),
		"beam:resources:accelerator:v1":   []byte("type:jeans;count1;"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("hs.Payloads() = %v, want %v", got, want)
	}
}

func TestHints_NilHints(t *testing.T) {
	var hs1, hs2 Hints

	hs := NewHints(MinRamBytes(2e9), Accelerator("type:pants;count1;install-pajamas"))

	if got, want := hs1.Equal(hs2), true; got != want {
		t.Errorf("nils equal test: (nil).Equal(nil) = %v, want %v", got, want)
	}
	if got, want := hs.Equal(hs2), false; got != want {
		t.Errorf("nil equal test: hs.Equal(nil) = %v, want %v", got, want)
	}
	if got, want := hs1.Equal(hs), false; got != want {
		t.Errorf("nil equal test: (nil).Equal(hs) = %v, want %v", got, want)
	}

	if got, want := hs1.MergeWithOuter(hs2), (Hints{}); !got.Equal(want) {
		t.Errorf("nils equal test: (nil).Equal(nil) = %v, want %v", got, want)
	}
	if got, want := hs.MergeWithOuter(hs2), hs; !got.Equal(want) {
		t.Errorf("nil equal test: hs.Equal(nil) = %v, want %v", got, want)
	}
	if got, want := hs1.MergeWithOuter(hs), hs; !got.Equal(want) {
		t.Errorf("nil equal test: (nil).Equal(hs) = %v, want %v", got, want)
	}
}
