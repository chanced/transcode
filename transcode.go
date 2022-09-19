// Package transcode is a library that will convert markup languages (currently
// JSON and YAML) from one form to another.
package transcode

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/tidwall/gjson"
	yaml "gopkg.in/yaml.v3"
)

var bufPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

var (
	newline      = []byte{'\n'}
	quotation    = []byte{'"'}
	comma        = []byte{','}
	leftBracket  = []byte{'['}
	rightBracket = []byte{']'}
	leftCurly    = []byte{'{'}
	rightCurly   = []byte{'}'}
	colon        = []byte{':'}
)

func New(w io.Writer) Transcoder {
	return Transcoder{
		w:      w,
		Indent: 4,
	}
}

type Transcoder struct {
	Indent  int
	w       io.Writer
	written bool
}

// JSONFromYAML transcodes YAML contained in yamlData
//
// Multiple documents are not supported.
func (t Transcoder) JSONFromYAML(yamlData io.Reader) error {
	dec := yaml.NewDecoder(yamlData)
	var n yaml.Node
	if err := dec.Decode(&n); err != nil {
		return err
	}
	yn := yamlnode{&n}
	return yn.encode(t.w)
}

// YAMLFromJSON transcodes JSON contained in jsonData into YAML
//
// Transcoding multiple items are not supported. Each root record will be seperated
// by "---"
func (t Transcoder) YAMLFromJSON(jsonData io.Reader) error {
	var o json.RawMessage
	dec := json.NewDecoder(jsonData)
	dec.UseNumber()

	if err := dec.Decode(&o); err != nil {
		return err
	}

	in := bytes.Repeat([]byte{' '}, t.Indent)
	err := jsonnode(o).encode(t.w, gjson.ParseBytes(o), 0, in)
	if err != nil {
		return err
	}
	for dec.More() {
		t.w.Write(newline)
		t.w.Write([]byte("---"))
		t.w.Write(newline)
		if err := dec.Decode(&o); err != nil {
			return err
		}
		err = jsonnode(o).encode(t.w, gjson.ParseBytes(o), 0, in)
		if err != nil {
			return err
		}
	}
	return nil
}

func JSONFromYAML(yamlData []byte) ([]byte, error) {
	var yn yaml.Node
	err := yaml.Unmarshal(yamlData, &yn)
	if err != nil {
		return nil, err
	}
	y := yamlnode{&yn}
	b := bufPool.Get().(*bytes.Buffer)
	defer bufPool.Put(b)
	b.Reset()
	b.Grow(len(yamlData))

	if err = y.EncodeJSON(b); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func YAMLFromJSON(jsonData []byte) ([]byte, error) {
	// going to do this twice to improve formatting
	return jsonnode(jsonData).MarshalYAML()
}

type yamlnode struct{ *yaml.Node }

func (y yamlnode) EncodeJSON(w io.Writer) error {
	return y.encode(w)
}

func (y yamlnode) encode(w io.Writer) error {
	switch y.Kind {
	case yaml.DocumentNode:
		return y.encodeDocument(w)
	case yaml.ScalarNode:
		return y.encodeScalar(w)
	case yaml.SequenceNode:
		return y.encodeSequence(w)
	case yaml.MappingNode:
		return y.encodeMapping(w)
	case yaml.AliasNode:
		return fmt.Errorf("aliases are not currently supported")
	default:
		return fmt.Errorf("unknown node kind: %d", y.Kind)
	}
}

func (y yamlnode) encodeDocument(w io.Writer) error {
	for _, v := range y.Content {
		return yamlnode{v}.EncodeJSON(w)
	}
	return nil
}

func (y yamlnode) encodeMapping(w io.Writer) error {
	if len(y.Content)%2 != 0 {
		return fmt.Errorf("mapping node has odd number of children")
	}
	w.Write(leftCurly)
	var err error
	var k []byte
	for i := 0; i < len(y.Content); i += 2 {
		if i > 0 {
			w.Write(comma)
		}
		k, err = yamlnode{y.Content[i]}.encodeKey()
		if err != nil {
			return err
		}
		w.Write(k)
		w.Write(colon)
		err := yamlnode{y.Content[i+1]}.encode(w)
		if err != nil {
			return err
		}
	}
	w.Write(rightCurly)
	return nil
}

func (y yamlnode) encodeKey() ([]byte, error) {
	switch y.Kind {
	case yaml.ScalarNode:
		sd, err := json.Marshal(y.Value)
		if err != nil {
			return nil, err
		}
		return sd, nil
	default:
		return nil, fmt.Errorf("unknown node key kind: %d", y.Kind)
	}
}

func (y yamlnode) encodeSequence(w io.Writer) error {
	w.Write(leftBracket)
	var err error

	for i, v := range y.Content {
		if i > 0 {
			w.Write(comma)
		}
		err = yamlnode{v}.encode(w)
		if err != nil {
			return err
		}
	}
	w.Write(rightBracket)
	return nil
}

func (y yamlnode) encodeScalar(w io.Writer) error {
	switch y.Tag {
	case "!!str":
		b, err := y.encodeString()
		if err != nil {
			return err
		}
		_, err = w.Write(b)
		return err
	case "!!int", "!!float":
		b, err := y.encodeNumber()
		if err != nil {
			return err
		}
		_, err = w.Write(b)
		return err
	case "!!bool":
		w.Write([]byte(y.Value))
		return nil
	case "!!null":
		w.Write([]byte("null"))
		return nil
	}
	return fmt.Errorf("unknown scalar tag: %q", y.Tag)
}

func (y yamlnode) encodeString() ([]byte, error) {
	return json.Marshal(y.Value)
}

func (y yamlnode) encodeNumber() ([]byte, error) {
	if isNumber([]byte(y.Value)) {
		return json.Marshal(json.Number(y.Value))
	}
	return json.Marshal(y.Value)
}

type jsonnode json.RawMessage

// MarshalYAML implements yaml.BytesMarshaler
func (j jsonnode) MarshalYAML() ([]byte, error) {
	b := bufPool.Get().(*bytes.Buffer)
	b.Reset()
	defer bufPool.Put(b)
	b.Grow(len(j) * 2)
	err := j.EncodeYAML(b)
	if err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func (j jsonnode) EncodeYAML(w io.Writer) error {
	return j.encode(w, gjson.ParseBytes(j), 0, []byte("    "))
}

func (j jsonnode) encode(w io.Writer, r gjson.Result, indent int, indention []byte) error {
	switch r.Type {
	case gjson.Null:
		w.Write([]byte("null"))
		return nil
	case gjson.False:
		w.Write([]byte("false"))
		return nil
	case gjson.True:
		w.Write([]byte("true"))
		return nil
	case gjson.Number:
		w.Write([]byte(r.Raw))
	case gjson.String:
		return j.encodeString(w, []byte(r.String()), indent, indention)
	}
	if r.IsArray() {
		return j.encodeArray(w, r, indent, indention)
	}
	if r.IsObject() {
		return j.encodeObject(w, r, indent, indention)
	}
	return nil
}

func isBool(k []byte) bool {
	return bytes.Equal(k, []byte("true")) || bytes.Equal(k, []byte("false"))
}

func isYesNo(k []byte) bool {
	return bytes.Equal(k, []byte("yes")) || bytes.Equal(k, []byte("no"))
}

func writeYAMLKey(w io.Writer, k []byte) {
	quote := isNumber(k) || isBool(k) || isYesNo(k)
	if quote {
		w.Write(quotation)
	}
	w.Write(k)
	if quote {
		w.Write(quotation)
	}
}

func writeIndention(w io.Writer, s []byte, i int) {
	for j := 0; j < i; j++ {
		w.Write(s)
	}
}

func (j jsonnode) encodeObject(w io.Writer, r gjson.Result, indent int, indention []byte) error {
	var err error
	i := 0
	r.ForEach(func(key, value gjson.Result) bool {
		if i > 0 || indent > 0 {
			w.Write(newline)
			writeIndention(w, indention, indent)
		}
		i += 1

		writeYAMLKey(w, []byte(key.String()))
		w.Write([]byte(": "))

		err = j.encode(w, value, indent+1, indention)
		if err != nil {
			return false
		}

		return err == nil
	})

	return nil
}

func (j jsonnode) encodeArray(w io.Writer, r gjson.Result, indent int, indention []byte) error {
	var err error
	i := 0

	r.ForEach(func(_, value gjson.Result) bool {
		if i > 0 || indent > 0 {
			w.Write([]byte("\n"))
			writeIndention(w, indention, indent)
		}
		i += 1
		w.Write([]byte("- "))
		err = j.encode(w, value, indent+1, indention)
		if err != nil {
			return false
		}
		return err == nil
	})
	return nil
}

func (j jsonnode) encodeString(w io.Writer, d []byte, indent int, indention []byte) error {
	switch {
	case isNumber(d) || isBool(d) || isYesNo(d) || bytes.ContainsAny(d, "\t#\\n"):
		w.Write(quotation)
		w.Write(d)
		w.Write(quotation)
		return nil
	case bytes.ContainsAny(d, "\n"):
		w.Write([]byte{'|'})
		if !bytes.HasSuffix(d, []byte("\n")) {
			w.Write([]byte{'-'})
		}
		s := bytes.Split(d, []byte("\n"))
		for _, l := range s {
			w.Write(newline)
			writeIndention(w, indention, indent+1)
			w.Write(l)
		}
		return nil
	default:
		w.Write(d)
		return nil
	}
}

// func (j jsonnode) encodeStringBlock(r gjson.Result, indent int) ([]byte, error) {
// 	panic("not impl")
// }

// IsValid reports whether s is a valid JSON number literal.
//
// Taken from encoding/json/scanner.go
func isNumber(data []byte) bool {
	// This function implements the JSON numbers grammar.
	// See https://tools.ietf.org/html/rfc7159#section-6
	// and https://www.json.org/img/number.png

	if len(data) == 0 {
		return false
	}

	// Optional -
	if data[0] == '-' {
		data = data[1:]
		if len(data) == 0 {
			return false
		}
	}

	// Digits
	switch {
	default:
		return false

	case data[0] == '0':
		data = data[1:]

	case '1' <= data[0] && data[0] <= '9':
		data = data[1:]
		for len(data) > 0 && '0' <= data[0] && data[0] <= '9' {
			data = data[1:]
		}
	}

	// . followed by 1 or more digits.
	if len(data) >= 2 && data[0] == '.' && '0' <= data[1] && data[1] <= '9' {
		data = data[2:]
		for len(data) > 0 && '0' <= data[0] && data[0] <= '9' {
			data = data[1:]
		}
	}

	// e or E followed by an optional - or + and
	// 1 or more digits.
	if len(data) >= 2 && (data[0] == 'e' || data[0] == 'E') {
		data = data[1:]
		if data[0] == '+' || data[0] == '-' {
			data = data[1:]
			if len(data) == 0 {
				return false
			}
		}
		for len(data) > 0 && '0' <= data[0] && data[0] <= '9' {
			data = data[1:]
		}
	}

	// Make sure we are at the end.
	return len(data) == 0
}
