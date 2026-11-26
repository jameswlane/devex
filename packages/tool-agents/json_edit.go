package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type jsonValue interface {
	jsonEqual(other jsonValue) bool
	writeJSON(buf *bytes.Buffer, indent int)
}

type jsonEntry struct {
	Key   string
	Value jsonValue
}

type jsonObject struct {
	Entries []jsonEntry
}

type jsonArray struct {
	Items []jsonValue
}

type jsonRaw struct {
	Raw string
}

func newJSONString(s string) jsonRaw {
	b, _ := json.Marshal(s)
	return jsonRaw{Raw: string(b)}
}

func (r jsonRaw) asString() (string, bool) {
	if len(r.Raw) < 2 || r.Raw[0] != '"' {
		return "", false
	}
	var s string
	if err := json.Unmarshal([]byte(r.Raw), &s); err != nil {
		return "", false
	}
	return s, true
}

func (r jsonRaw) jsonEqual(other jsonValue) bool {
	o, ok := other.(jsonRaw)
	if !ok {
		return false
	}
	if r.Raw == o.Raw {
		return true
	}
	// Compare semantically for strings with differing escape forms.
	rs, rOK := r.asString()
	os, oOK := o.asString()
	return rOK && oOK && rs == os
}

func (r jsonRaw) writeJSON(buf *bytes.Buffer, _ int) {
	buf.WriteString(r.Raw)
}

func (a *jsonArray) jsonEqual(other jsonValue) bool {
	o, ok := other.(*jsonArray)
	if !ok || len(a.Items) != len(o.Items) {
		return false
	}
	for i := range a.Items {
		if !a.Items[i].jsonEqual(o.Items[i]) {
			return false
		}
	}
	return true
}

func (a *jsonArray) writeJSON(buf *bytes.Buffer, indent int) {
	if len(a.Items) == 0 {
		buf.WriteString("[]")
		return
	}
	buf.WriteString("[\n")
	for i, item := range a.Items {
		writeIndent(buf, indent+2)
		item.writeJSON(buf, indent+2)
		if i < len(a.Items)-1 {
			buf.WriteByte(',')
		}
		buf.WriteByte('\n')
	}
	writeIndent(buf, indent)
	buf.WriteByte(']')
}

func (o *jsonObject) Get(key string) (jsonValue, bool) {
	for _, e := range o.Entries {
		if e.Key == key {
			return e.Value, true
		}
	}
	return nil, false
}

func (o *jsonObject) Set(key string, val jsonValue) {
	for i := range o.Entries {
		if o.Entries[i].Key == key {
			o.Entries[i].Value = val
			return
		}
	}
	o.Entries = append(o.Entries, jsonEntry{Key: key, Value: val})
}

func (o *jsonObject) EnsureObject(key string) (*jsonObject, error) {
	if existing, ok := o.Get(key); ok {
		obj, isObj := existing.(*jsonObject)
		if !isObj {
			return nil, fmt.Errorf("key %q is not a JSON object", key)
		}
		return obj, nil
	}
	child := &jsonObject{}
	o.Set(key, child)
	return child, nil
}

func (o *jsonObject) GetStringArray(key string) ([]string, bool, error) {
	existing, ok := o.Get(key)
	if !ok {
		return nil, false, nil
	}
	arr, isArr := existing.(*jsonArray)
	if !isArr {
		return nil, true, fmt.Errorf("key %q is not a JSON array", key)
	}
	out := make([]string, 0, len(arr.Items))
	for i, item := range arr.Items {
		raw, isRaw := item.(jsonRaw)
		if !isRaw {
			return nil, true, fmt.Errorf("key %q[%d] is not a JSON string", key, i)
		}
		s, isStr := raw.asString()
		if !isStr {
			return nil, true, fmt.Errorf("key %q[%d] is not a JSON string", key, i)
		}
		out = append(out, s)
	}
	return out, true, nil
}

func (o *jsonObject) SetStringArray(key string, items []string) {
	arr := &jsonArray{Items: make([]jsonValue, 0, len(items))}
	for _, s := range items {
		arr.Items = append(arr.Items, newJSONString(s))
	}
	o.Set(key, arr)
}

func (o *jsonObject) Clone() *jsonObject {
	clone := &jsonObject{Entries: make([]jsonEntry, len(o.Entries))}
	for i, e := range o.Entries {
		clone.Entries[i] = jsonEntry{Key: e.Key, Value: cloneJSONValue(e.Value)}
	}
	return clone
}

func cloneJSONValue(v jsonValue) jsonValue {
	switch x := v.(type) {
	case *jsonObject:
		return x.Clone()
	case *jsonArray:
		items := make([]jsonValue, len(x.Items))
		for i, item := range x.Items {
			items[i] = cloneJSONValue(item)
		}
		return &jsonArray{Items: items}
	case jsonRaw:
		return x
	default:
		return v
	}
}

func (o *jsonObject) jsonEqual(other jsonValue) bool {
	o2, ok := other.(*jsonObject)
	if !ok || len(o.Entries) != len(o2.Entries) {
		return false
	}
	for i := range o.Entries {
		if o.Entries[i].Key != o2.Entries[i].Key {
			return false
		}
		if !o.Entries[i].Value.jsonEqual(o2.Entries[i].Value) {
			return false
		}
	}
	return true
}

func (o *jsonObject) writeJSON(buf *bytes.Buffer, indent int) {
	if len(o.Entries) == 0 {
		buf.WriteString("{}")
		return
	}
	buf.WriteString("{\n")
	for i, entry := range o.Entries {
		writeIndent(buf, indent+2)
		keyBytes, _ := json.Marshal(entry.Key)
		buf.Write(keyBytes)
		buf.WriteString(": ")
		entry.Value.writeJSON(buf, indent+2)
		if i < len(o.Entries)-1 {
			buf.WriteByte(',')
		}
		buf.WriteByte('\n')
	}
	writeIndent(buf, indent)
	buf.WriteByte('}')
}

func (o *jsonObject) MarshalIndent() string {
	var buf bytes.Buffer
	o.writeJSON(&buf, 0)
	buf.WriteByte('\n')
	return buf.String()
}

func writeIndent(buf *bytes.Buffer, n int) {
	for i := 0; i < n; i++ {
		buf.WriteByte(' ')
	}
}

// ParseJSONObject parses strict RFC 8259 JSON whose root value is an object,
// preserving exact key order and failing clearly on comments, trailing commas,
// or invalid syntax.
func ParseJSONObject(data []byte, path string) (*jsonObject, error) {
	p := &jsonParser{src: string(data), path: path}
	if err := p.skipWS(); err != nil {
		return nil, err
	}
	if p.pos >= len(p.src) {
		return nil, fmt.Errorf("parse %s: empty JSON file", path)
	}
	val, err := p.parseValue()
	if err != nil {
		return nil, err
	}
	if err := p.skipWS(); err != nil {
		return nil, err
	}
	if p.pos < len(p.src) {
		return nil, fmt.Errorf("parse %s: unexpected trailing character %q at offset %d", path, p.src[p.pos], p.pos)
	}
	obj, ok := val.(*jsonObject)
	if !ok {
		return nil, fmt.Errorf("parse %s: top-level JSON value must be an object", path)
	}
	return obj, nil
}

type jsonParser struct {
	src  string
	pos  int
	path string
}

func (p *jsonParser) skipWS() error {
	for p.pos < len(p.src) {
		c := p.src[p.pos]
		if c == ' ' || c == '\t' || c == '\r' || c == '\n' {
			p.pos++
			continue
		}
		if c == '/' && p.pos+1 < len(p.src) && (p.src[p.pos+1] == '/' || p.src[p.pos+1] == '*') {
			return fmt.Errorf("parse %s: file contains JSON comments; DevEx will not strip comments — remove comments or manage this file manually", p.path)
		}
		break
	}
	return nil
}

func (p *jsonParser) parseValue() (jsonValue, error) {
	if err := p.skipWS(); err != nil {
		return nil, err
	}
	if p.pos >= len(p.src) {
		return nil, fmt.Errorf("parse %s: unexpected end of JSON", p.path)
	}
	switch p.src[p.pos] {
	case '{':
		return p.parseObject()
	case '[':
		return p.parseArray()
	case '"':
		raw, err := p.parseStringRaw()
		if err != nil {
			return nil, err
		}
		return jsonRaw{Raw: raw}, nil
	case 't':
		return p.parseLiteral("true")
	case 'f':
		return p.parseLiteral("false")
	case 'n':
		return p.parseLiteral("null")
	default:
		if p.src[p.pos] == '-' || (p.src[p.pos] >= '0' && p.src[p.pos] <= '9') {
			return p.parseNumber()
		}
		return nil, fmt.Errorf("parse %s: unexpected character %q at offset %d", p.path, p.src[p.pos], p.pos)
	}
}

func (p *jsonParser) parseObject() (*jsonObject, error) {
	p.pos++ // '{'
	obj := &jsonObject{}
	if err := p.skipWS(); err != nil {
		return nil, err
	}
	if p.pos < len(p.src) && p.src[p.pos] == '}' {
		p.pos++
		return obj, nil
	}
	for {
		if err := p.skipWS(); err != nil {
			return nil, err
		}
		if p.pos >= len(p.src) {
			return nil, fmt.Errorf("parse %s: unterminated JSON object", p.path)
		}
		if p.src[p.pos] == '}' {
			return nil, fmt.Errorf("parse %s: trailing comma in JSON object; DevEx will not rewrite non-standard JSONC", p.path)
		}
		if p.src[p.pos] != '"' {
			return nil, fmt.Errorf("parse %s: expected object key string at offset %d", p.path, p.pos)
		}
		rawKey, err := p.parseStringRaw()
		if err != nil {
			return nil, err
		}
		var key string
		if err := json.Unmarshal([]byte(rawKey), &key); err != nil {
			return nil, fmt.Errorf("parse %s: invalid string key: %w", p.path, err)
		}
		if err := p.skipWS(); err != nil {
			return nil, err
		}
		if p.pos >= len(p.src) || p.src[p.pos] != ':' {
			return nil, fmt.Errorf("parse %s: expected ':' after key %q at offset %d", p.path, key, p.pos)
		}
		p.pos++
		val, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		obj.Set(key, val)
		if err := p.skipWS(); err != nil {
			return nil, err
		}
		if p.pos >= len(p.src) {
			return nil, fmt.Errorf("parse %s: unterminated JSON object", p.path)
		}
		switch p.src[p.pos] {
		case ',':
			p.pos++
		case '}':
			p.pos++
			return obj, nil
		default:
			return nil, fmt.Errorf("parse %s: expected ',' or '}' at offset %d", p.path, p.pos)
		}
	}
}

func (p *jsonParser) parseArray() (*jsonArray, error) {
	p.pos++ // '['
	arr := &jsonArray{}
	if err := p.skipWS(); err != nil {
		return nil, err
	}
	if p.pos < len(p.src) && p.src[p.pos] == ']' {
		p.pos++
		return arr, nil
	}
	for {
		if err := p.skipWS(); err != nil {
			return nil, err
		}
		if p.pos < len(p.src) && p.src[p.pos] == ']' {
			return nil, fmt.Errorf("parse %s: trailing comma in JSON array; DevEx will not rewrite non-standard JSONC", p.path)
		}
		val, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		arr.Items = append(arr.Items, val)
		if err := p.skipWS(); err != nil {
			return nil, err
		}
		if p.pos >= len(p.src) {
			return nil, fmt.Errorf("parse %s: unterminated JSON array", p.path)
		}
		switch p.src[p.pos] {
		case ',':
			p.pos++
		case ']':
			p.pos++
			return arr, nil
		default:
			return nil, fmt.Errorf("parse %s: expected ',' or ']' at offset %d", p.path, p.pos)
		}
	}
}

func (p *jsonParser) parseStringRaw() (string, error) {
	start := p.pos
	p.pos++ // opening '"'
	for p.pos < len(p.src) {
		c := p.src[p.pos]
		if c == '\\' {
			p.pos += 2
			continue
		}
		if c == '"' {
			p.pos++
			raw := p.src[start:p.pos]
			var check string
			if err := json.Unmarshal([]byte(raw), &check); err != nil {
				return "", fmt.Errorf("parse %s: invalid JSON string at offset %d: %w", p.path, start, err)
			}
			return raw, nil
		}
		if c < 0x20 {
			return "", fmt.Errorf("parse %s: unescaped control character in string at offset %d", p.path, p.pos)
		}
		p.pos++
	}
	return "", fmt.Errorf("parse %s: unterminated JSON string at offset %d", p.path, start)
}

func (p *jsonParser) parseLiteral(lit string) (jsonRaw, error) {
	if strings.HasPrefix(p.src[p.pos:], lit) {
		p.pos += len(lit)
		return jsonRaw{Raw: lit}, nil
	}
	return jsonRaw{}, fmt.Errorf("parse %s: invalid JSON literal at offset %d", p.path, p.pos)
}

func (p *jsonParser) parseNumber() (jsonRaw, error) {
	start := p.pos
	if p.src[p.pos] == '-' {
		p.pos++
	}
	for p.pos < len(p.src) {
		c := p.src[p.pos]
		if (c >= '0' && c <= '9') || c == '.' || c == 'e' || c == 'E' || c == '+' || c == '-' {
			p.pos++
			continue
		}
		break
	}
	raw := p.src[start:p.pos]
	var num json.Number
	if err := json.Unmarshal([]byte(raw), &num); err != nil {
		return jsonRaw{}, fmt.Errorf("parse %s: invalid JSON number %q at offset %d", p.path, raw, start)
	}
	// Reject leading zeros such as 01 which encoding/json might accept via Number.
	if _, err := strconv.ParseFloat(raw, 64); err != nil {
		return jsonRaw{}, fmt.Errorf("parse %s: invalid JSON number %q at offset %d", p.path, raw, start)
	}
	if !json.Valid([]byte(raw)) {
		return jsonRaw{}, fmt.Errorf("parse %s: invalid JSON number %q at offset %d", p.path, raw, start)
	}
	return jsonRaw{Raw: raw}, nil
}
