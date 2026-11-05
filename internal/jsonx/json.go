// Package jsonx implements the strict JSON subset shared by configuration and HTTP.
package jsonx

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
	"unicode/utf8"
)

const MaxInteger uint64 = 1<<53 - 1

func Decode(raw []byte, out any) error {
	if !utf8.Valid(raw) || !json.Valid(raw) || !validEscapes(raw) {
		return errors.New("invalid JSON encoding")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	value, err := read(d, 0)
	if err != nil {
		return err
	}
	if _, err = d.Token(); err != io.EOF {
		return errors.New("trailing JSON")
	}
	if err = shape(value, reflect.TypeOf(out).Elem()); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	return decoder.Decode(out)
}
func read(d *json.Decoder, depth int) (any, error) {
	if depth > 24 {
		return nil, errors.New("JSON nesting limit")
	}
	token, err := d.Token()
	if err != nil {
		return nil, err
	}
	switch v := token.(type) {
	case json.Delim:
		if v == '{' {
			m := map[string]any{}
			for d.More() {
				k, e := d.Token()
				if e != nil {
					return nil, e
				}
				s := k.(string)
				if _, ok := m[s]; ok {
					return nil, errors.New("duplicate JSON key")
				}
				x, e := read(d, depth+1)
				if e != nil {
					return nil, e
				}
				m[s] = x
			}
			_, err = d.Token()
			return m, err
		}
		if v == '[' {
			var a []any
			for d.More() {
				x, e := read(d, depth+1)
				if e != nil {
					return nil, e
				}
				a = append(a, x)
			}
			_, err = d.Token()
			return a, err
		}
		return nil, errors.New("invalid delimiter")
	case json.Number:
		s := string(v)
		if strings.ContainsAny(s, ".eE") {
			return nil, errors.New("integer required")
		}
		n, e := strconv.ParseInt(s, 10, 64)
		if e != nil || n > int64(MaxInteger) || n < -int64(MaxInteger) {
			return nil, errors.New("integer outside safe range")
		}
		return v, nil
	default:
		return token, nil
	}
}
func shape(v any, t reflect.Type) error {
	if t.Kind() == reflect.Pointer {
		if v == nil {
			return nil
		}
		return shape(v, t.Elem())
	}
	if v == nil {
		return errors.New("unexpected null")
	}
	switch t.Kind() {
	case reflect.Struct:
		m, ok := v.(map[string]any)
		if !ok {
			return errors.New("object required")
		}
		known := map[string]bool{}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			tag := f.Tag.Get("json")
			if tag == "-" || f.PkgPath != "" {
				continue
			}
			name := strings.Split(tag, ",")[0]
			if name == "" {
				name = f.Name
			}
			known[name] = true
			x, exists := m[name]
			if !exists {
				if strings.Contains(tag, ",omitempty") {
					continue
				}
				return fmt.Errorf("missing field %s", name)
			}
			if err := shape(x, f.Type); err != nil {
				return fmt.Errorf("field %s: %w", name, err)
			}
		}
		for k := range m {
			if !known[k] {
				return fmt.Errorf("unknown field %s", k)
			}
		}
	case reflect.Slice, reflect.Array:
		if t.Elem().Kind() == reflect.Uint8 {
			return nil
		}
		a, ok := v.([]any)
		if !ok {
			return errors.New("array required")
		}
		for _, x := range a {
			if err := shape(x, t.Elem()); err != nil {
				return err
			}
		}
	}
	return nil
}
func validEscapes(raw []byte) bool {
	for i := 0; i < len(raw); i++ {
		if raw[i] != '\\' {
			continue
		}
		i++
		if i >= len(raw) {
			return false
		}
		if raw[i] != 'u' {
			continue
		}
		if i+4 >= len(raw) {
			return false
		}
		n, e := strconv.ParseUint(string(raw[i+1:i+5]), 16, 16)
		if e != nil {
			return false
		}
		i += 4
		if n >= 0xD800 && n <= 0xDBFF {
			if i+6 >= len(raw) || raw[i+1] != '\\' || raw[i+2] != 'u' {
				return false
			}
			low, e := strconv.ParseUint(string(raw[i+3:i+7]), 16, 16)
			if e != nil || low < 0xDC00 || low > 0xDFFF {
				return false
			}
			i += 6
		} else if n >= 0xDC00 && n <= 0xDFFF {
			return false
		}
	}
	return true
}
