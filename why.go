package why

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	yaml "gopkg.in/yaml.v3"
)

const INDENTION string = "    "

func YAMLToJSON(yamlData []byte) ([]byte, error) {
	var yn yaml.Node
	err := yaml.Unmarshal(yamlData, &yn)
	if err != nil {
		return nil, err
	}
	y := yamlnode{&yn}
	return y.MarshalJSON()
}

func JSONToYAML(jsonData []byte) ([]byte, error) {
	// going to do this twice to improve formatting
	return jsonnode(jsonData).MarshalYAML()
}

type yamlnode struct{ *yaml.Node }

func (y yamlnode) MarshalJSON() ([]byte, error) {
	return y.encode()
}

func (y yamlnode) encode() ([]byte, error) {
	switch y.Kind {
	case yaml.DocumentNode:
		return y.encodeDocument()
	case yaml.ScalarNode:
		return y.encodeScalar()
	case yaml.SequenceNode:
		return y.encodeSequence()
	case yaml.MappingNode:
		return y.encodeMapping()
	case yaml.AliasNode:
		return nil, fmt.Errorf("aliases are not currently supported")
	default:
		return nil, fmt.Errorf("unknown node kind: %d", y.Kind)
	}
}

func (y yamlnode) encodeDocument() ([]byte, error) {
	for _, v := range y.Content {
		return yamlnode{v}.MarshalJSON()
	}
	return nil, nil
}

func (y yamlnode) encodeMapping() ([]byte, error) {
	if len(y.Content)%2 != 0 {
		return nil, fmt.Errorf("mapping node has odd number of children")
	}
	b := []byte("{}")
	for i := 0; i < len(y.Content); i += 2 {
		k, err := yamlnode{y.Content[i]}.key()
		if err != nil {
			return nil, err
		}
		v, err := yamlnode{y.Content[i+1]}.encode()
		if err != nil {
			return nil, err
		}
		b, err = sjson.SetRawBytes(b, string(k), v)
		if err != nil {
			return nil, err
		}
	}

	return b, nil
}

func (y yamlnode) key() (string, error) {
	switch y.Kind {
	case yaml.ScalarNode:
		return y.Value, nil
	default:
		return "", fmt.Errorf("unknown node key kind: %d", y.Kind)
	}
}

func (y yamlnode) encodeSequence() ([]byte, error) {
	var x []byte
	b := []byte("[]")
	var err error
	for _, v := range y.Content {
		x, err = yamlnode{v}.encode()
		if err != nil {
			return nil, err
		}
		b, err = sjson.SetRawBytes(b, "-1", x)

		if err != nil {
			return nil, err
		}
	}
	return b, nil
}

func (y yamlnode) encodeScalar() ([]byte, error) {
	switch y.Tag {
	case "!!str":
		return y.encodeString()
	case "!!int", "!!float":
		return y.encodeNumber()
	case "!!bool":
		return []byte(y.Value), nil
	case "!!null":
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("unknown scalar tag: %q", y.Tag)
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
	r := gjson.ParseBytes(j)
	return j.encode(r, 0)
}

func (j jsonnode) encode(r gjson.Result, indent int) ([]byte, error) {
	switch r.Type {
	case gjson.Null:
		return []byte("null"), nil
	case gjson.False:
		return []byte("false"), nil
	case gjson.True:
		return []byte("true"), nil
	case gjson.Number:
		return []byte(r.Raw), nil
	case gjson.String:
		return j.encodeString(r, indent)
	}

	if r.IsArray() {
		return j.encodeArray(r, indent)
	}

	if r.IsObject() {
		return j.encodeObject(r, indent)
	}
	return nil, nil
}

func (j jsonnode) encodeObject(r gjson.Result, indent int) ([]byte, error) {
	b := strings.Builder{}
	var err error
	var x []byte
	r.ForEach(func(key, value gjson.Result) bool {
		if b.Len() > 0 || indent > 0 {
			b.WriteByte('\n')
			for i := 0; i < indent; i++ {
				b.WriteString(INDENTION)
			}
		}
		b.WriteString(key.String())

		b.WriteByte(':')
		b.WriteByte(' ')
		x, err = j.encode(value, indent+1)
		if err != nil {
			return false
		}
		_, err = b.Write(x)
		return err == nil
	})
	return []byte(b.String()), err
}

func (j jsonnode) encodeArray(r gjson.Result, indent int) ([]byte, error) {
	b := strings.Builder{}
	var err error
	var x []byte
	r.ForEach(func(_, value gjson.Result) bool {
		if b.Len() > 0 || indent > 0 {
			b.WriteByte('\n')
			for i := 0; i < indent; i++ {
				b.WriteString(INDENTION)
			}
		}
		b.WriteString("- ")
		x, err = j.encode(value, indent+1)
		if err != nil {
			return false
		}
		_, err = b.Write(x)
		return err == nil
	})
	return []byte(b.String()), err
}

func (j jsonnode) encodeString(r gjson.Result, indent int) ([]byte, error) {
	return json.Marshal(r.String())
	// newlineIndex := strings.Index(s, "\n")
	// if newlineIndex == -1 {
	// 	return json.Marshal(s)
	// }
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
