package routingevidence

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strconv"
	"unicode/utf8"
)

// canonicalJSON implements the package's single JSON grammar. It uses
// encoding/json only to project closed Go structs into JSON values, then writes
// those values itself so object ordering and escaping do not depend on struct
// declaration order or encoder HTML policy.
func canonicalJSON(v any) ([]byte, error) {
	if err := validUTF8Value(reflect.ValueOf(v)); err != nil {
		return nil, err
	}
	var projected bytes.Buffer
	enc := json.NewEncoder(&projected)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, fmt.Errorf("project canonical value: %w", err)
	}
	value, err := parseJSON(projected.Bytes())
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err := writeCanonical(&out, value); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func canonicalizeInput(b []byte) ([]byte, error) {
	if !utf8.Valid(b) {
		return nil, fmt.Errorf("canonical json: invalid UTF-8")
	}
	v, err := parseJSON(b)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err := writeCanonical(&out, v); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func parseJSON(b []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, fmt.Errorf("canonical json: %w", err)
	}
	if err := ensureEOF(dec); err != nil {
		return nil, err
	}
	return v, nil
}

func ensureEOF(dec *json.Decoder) error {
	var extra any
	err := dec.Decode(&extra)
	if err == io.EOF {
		return nil
	}
	if err == nil {
		return fmt.Errorf("canonical json: trailing value")
	}
	return fmt.Errorf("canonical json: trailing data: %w", err)
}

func writeCanonical(dst *bytes.Buffer, v any) error {
	switch x := v.(type) {
	case nil:
		return fmt.Errorf("canonical json: null is not admitted")
	case bool:
		if x {
			dst.WriteString("true")
		} else {
			dst.WriteString("false")
		}
	case string:
		writeJSONString(dst, x)
	case json.Number:
		if !canonicalInteger(string(x)) {
			return fmt.Errorf("canonical json: non-integer or non-minimal number %q", x)
		}
		dst.WriteString(string(x))
	case []any:
		dst.WriteByte('[')
		for i, item := range x {
			if i > 0 {
				dst.WriteByte(',')
			}
			if err := writeCanonical(dst, item); err != nil {
				return err
			}
		}
		dst.WriteByte(']')
	case map[string]any:
		keys := make([]string, 0, len(x))
		for key := range x {
			if !utf8.ValidString(key) {
				return fmt.Errorf("canonical json: invalid UTF-8 object key")
			}
			keys = append(keys, key)
		}
		sort.Strings(keys)
		dst.WriteByte('{')
		for i, key := range keys {
			if i > 0 {
				dst.WriteByte(',')
			}
			writeJSONString(dst, key)
			dst.WriteByte(':')
			if err := writeCanonical(dst, x[key]); err != nil {
				return err
			}
		}
		dst.WriteByte('}')
	default:
		return fmt.Errorf("canonical json: unsupported value %T", v)
	}
	return nil
}

func canonicalInteger(s string) bool {
	if s == "0" {
		return true
	}
	if len(s) == 0 {
		return false
	}
	start := 0
	if s[0] == '-' {
		if len(s) == 1 || s[1] == '0' {
			return false
		}
		start = 1
	} else if s[0] == '0' {
		return false
	}
	for i := start; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

func writeJSONString(dst *bytes.Buffer, s string) {
	dst.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"', '\\':
			dst.WriteByte('\\')
			dst.WriteRune(r)
		case '\b':
			dst.WriteString("\\b")
		case '\f':
			dst.WriteString("\\f")
		case '\n':
			dst.WriteString("\\n")
		case '\r':
			dst.WriteString("\\r")
		case '\t':
			dst.WriteString("\\t")
		default:
			if r < 0x20 {
				dst.WriteString("\\u00")
				dst.WriteString(strconv.FormatInt(int64(r)>>4, 16))
				dst.WriteString(strconv.FormatInt(int64(r)&0xf, 16))
			} else {
				dst.WriteRune(r)
			}
		}
	}
	dst.WriteByte('"')
}

func validUTF8Value(v reflect.Value) error {
	if !v.IsValid() {
		return nil
	}
	if v.Kind() == reflect.Interface || v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return nil
		}
		return validUTF8Value(v.Elem())
	}
	switch v.Kind() {
	case reflect.String:
		if !utf8.ValidString(v.String()) {
			return fmt.Errorf("canonical json: invalid UTF-8 string")
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if err := validUTF8Value(v.Field(i)); err != nil {
				return err
			}
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < v.Len(); i++ {
			if err := validUTF8Value(v.Index(i)); err != nil {
				return err
			}
		}
	case reflect.Map:
		return fmt.Errorf("canonical json: maps are not schema values")
	}
	return nil
}

func decodeClosed(b []byte, dst any) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	dec.UseNumber()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	return ensureEOF(dec)
}
