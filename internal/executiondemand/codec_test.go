package executiondemand

import (
	"bytes"
	"encoding/binary"
	"reflect"
	"testing"
)

func TestEncodeDecodeRoundTripPlanV2(t *testing.T) {
	o := baselineOriginV2()
	b, err := EncodeCanonical(o)
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeCanonical(b)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, o) {
		t.Fatalf("round trip mismatch:\ngot:  %+v\nwant: %+v", got, o)
	}
	b2, err := EncodeCanonical(got)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(b, b2) {
		t.Fatal("re-encoding a decoded origin produced different bytes")
	}
}

func TestEncodeDecodeRoundTripPlanV2NoRaise(t *testing.T) {
	p := baselinePlanV2()
	p.Raise = nil
	o := Origin{Schema: OriginSchema, Kind: OriginPlanV2, PlanV2: &p}
	b, err := EncodeCanonical(o)
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeCanonical(b)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, o) {
		t.Fatalf("round trip mismatch:\ngot:  %+v\nwant: %+v", got, o)
	}
}

func TestEncodeCanonicalRejectsInvalidOrigin(t *testing.T) {
	p := baselinePlanV2()
	p.Repository = ""
	if _, err := EncodeCanonical(Origin{Schema: OriginSchema, Kind: OriginPlanV2, PlanV2: &p}); err == nil {
		t.Fatal("invalid origin encoded successfully")
	}
}

func TestDecodeCanonicalFailsClosed(t *testing.T) {
	validBytes, err := EncodeCanonical(baselineOriginV2())
	if err != nil {
		t.Fatal(err)
	}

	t.Run("truncated", func(t *testing.T) {
		if _, err := DecodeCanonical(validBytes[:len(validBytes)-5]); err == nil {
			t.Fatal("truncated bytes accepted")
		}
	})
	t.Run("trailing_garbage", func(t *testing.T) {
		corrupt := append(append([]byte(nil), validBytes...), 0xFF)
		if _, err := DecodeCanonical(corrupt); err == nil {
			t.Fatal("trailing garbage accepted")
		}
	})
	t.Run("empty", func(t *testing.T) {
		if _, err := DecodeCanonical(nil); err == nil {
			t.Fatal("empty input accepted")
		}
	})
	t.Run("corrupted_first_tag", func(t *testing.T) {
		corrupt := append([]byte(nil), validBytes...)
		corrupt[0] = 0x09
		if _, err := DecodeCanonical(corrupt); err == nil {
			t.Fatal("corrupted leading tag accepted")
		}
	})
	t.Run("unknown_schema", func(t *testing.T) {
		o := baselineOriginV2()
		body := encodePlanV2Body(*o.PlanV2)
		b := writeFixedFields([][]byte{[]byte("execution-demand/v3"), []byte(o.Kind), body})
		if _, err := DecodeCanonical(b); err == nil {
			t.Fatal("unknown schema accepted")
		}
	})
	t.Run("unknown_kind", func(t *testing.T) {
		o := baselineOriginV2()
		body := encodePlanV2Body(*o.PlanV2)
		b := writeFixedFields([][]byte{[]byte(o.Schema), []byte("plan_v3"), body})
		if _, err := DecodeCanonical(b); err == nil {
			t.Fatal("unknown kind accepted")
		}
	})
	t.Run("mismatched_body_for_kind", func(t *testing.T) {
		o := baselineOriginV2()
		compatBody := []byte("retired legacy payload")
		b := writeFixedFields([][]byte{[]byte(o.Schema), []byte(o.Kind), compatBody})
		if _, err := DecodeCanonical(b); err == nil {
			t.Fatal("plan_v1_compat body decoded as declared plan_v2 kind")
		}
	})
}

func TestReadFixedFieldsRoundTrip(t *testing.T) {
	values := [][]byte{[]byte("a"), []byte(""), []byte("ccc")}
	b := writeFixedFields(values)
	got, err := readFixedFields(b, 3)
	if err != nil {
		t.Fatal(err)
	}
	for i := range values {
		if !bytes.Equal(got[i], values[i]) {
			t.Fatalf("field %d mismatch: got %q want %q", i, got[i], values[i])
		}
	}
}

func TestReadFixedFieldsRejectsDuplicateTag(t *testing.T) {
	var buf bytes.Buffer
	writeTag(&buf, 1, []byte("a"))
	writeTag(&buf, 1, []byte("b"))
	if _, err := readFixedFields(buf.Bytes(), 2); err == nil {
		t.Fatal("duplicate tag accepted")
	}
}

func TestReadFixedFieldsRejectsOutOfOrderTag(t *testing.T) {
	var buf bytes.Buffer
	writeTag(&buf, 2, []byte("b"))
	writeTag(&buf, 1, []byte("a"))
	if _, err := readFixedFields(buf.Bytes(), 2); err == nil {
		t.Fatal("out-of-order tag accepted")
	}
}

func TestReadFixedFieldsRejectsUnknownTag(t *testing.T) {
	var buf bytes.Buffer
	writeTag(&buf, 1, []byte("a"))
	writeTag(&buf, 99, []byte("b"))
	if _, err := readFixedFields(buf.Bytes(), 2); err == nil {
		t.Fatal("unknown tag accepted")
	}
}

func TestReadFixedFieldsRejectsMissingField(t *testing.T) {
	var buf bytes.Buffer
	writeTag(&buf, 1, []byte("a"))
	if _, err := readFixedFields(buf.Bytes(), 2); err == nil {
		t.Fatal("missing trailing field accepted")
	}
}

func TestReadFixedFieldsRejectsTrailingData(t *testing.T) {
	var buf bytes.Buffer
	writeTag(&buf, 1, []byte("a"))
	writeTag(&buf, 2, []byte("b"))
	buf.WriteByte(0xFF)
	if _, err := readFixedFields(buf.Bytes(), 2); err == nil {
		t.Fatal("trailing data accepted")
	}
}

func TestReadLDRejectsTruncation(t *testing.T) {
	if _, _, err := readLD([]byte{0, 0, 0, 0, 0, 0, 0}); err == nil {
		t.Fatal("truncated length prefix accepted")
	}
	lp := make([]byte, lenPrefixSize)
	binary.BigEndian.PutUint64(lp, 10)
	b := append(lp, []byte("ab")...)
	if _, _, err := readLD(b); err == nil {
		t.Fatal("truncated field value accepted")
	}
}

func TestEncodeStringListIsOrderIndependent(t *testing.T) {
	a := encodeStringList([]string{"b", "a", "c"})
	b := encodeStringList([]string{"c", "b", "a"})
	if !bytes.Equal(a, b) {
		t.Fatal("string list encoding depends on input order")
	}
}

func TestEncodeRulePairsIsOrderIndependent(t *testing.T) {
	rs1 := []AppliedRule{{ID: "b", Kind: "floor"}, {ID: "a", Kind: "promotion"}}
	rs2 := []AppliedRule{{ID: "a", Kind: "promotion"}, {ID: "b", Kind: "floor"}}
	if !bytes.Equal(encodeRulePairs(rs1), encodeRulePairs(rs2)) {
		t.Fatal("rule pair encoding depends on input order")
	}
}
