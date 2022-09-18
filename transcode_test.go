package transcode_test

import (
	"bytes"
	"embed"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chanced/transcode"
	"github.com/google/go-cmp/cmp"
	"gopkg.in/yaml.v3"
)

//go:embed testdata
var testdata embed.FS

func Test(t *testing.T) {
	fs.WalkDir(testdata, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			t.Fatal(t)
		}
		if d.IsDir() {
			return nil
		}
		name := strings.TrimSuffix(p, filepath.Ext(p))
		name = strings.TrimPrefix(name, "testdata/")

		t.Run(name, func(t *testing.T) {
			yamlData, err := testdata.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			var expectedYAML interface{}
			err = yaml.Unmarshal(yamlData, &expectedYAML)
			if err != nil {
				t.Fatal(err)
			}

			jsonData, err := transcode.JSONFromYAML(yamlData)
			if err != nil {
				t.Fatal(err)
			}

			var actualJSON interface{}
			err = json.Unmarshal(jsonData, &actualJSON)
			if err != nil {
				t.Fatal(err)
			}

			yamlFromJSON, err := transcode.YAMLFromJSON(jsonData)
			if err != nil {
				t.Error(err)
			}
			os.MkdirAll(filepath.Dir("testoutput/"+name), 0o755)
			err = os.WriteFile("testoutput/"+name+".yaml", yamlFromJSON, 0o644)
			if err != nil {
				t.Fatal(err)
			}
			var actualYAML interface{}
			err = yaml.Unmarshal(yamlFromJSON, &actualYAML)
			if err != nil {
				t.Error(err)
			}

			if diff := cmp.Diff(expectedYAML, actualYAML); diff != "" {
				t.Errorf("yaml mismatch:\n%s", diff)
			}

			yamlr, err := testdata.Open(p)
			if err != nil {
				t.Fatal(err)
			}
			_ = yamlr

			jsonr := bytes.NewBuffer(jsonData)
			jsonbuf := bytes.Buffer{}
			yamlbuf := bytes.Buffer{}

			_ = jsonbuf
			_ = yamlbuf

			tr := transcode.New(&yamlbuf)
			if err = tr.YAMLFromJSON(jsonr); err != nil {
				t.Error(err)
			}

			// err = os.WriteFile("testoutput/"+name+"_encoder.yaml", yamlbuf.Bytes(), 0o644)
			// if err != nil {
			// 	t.Fatal(err)
			// }

			if !cmp.Equal(yamlFromJSON, yamlbuf.Bytes()) {
				t.Errorf("yaml mismatch:\n%s", cmp.Diff(yamlFromJSON, yamlbuf.Bytes()))
			}

			tr = transcode.New(&jsonbuf)
			err = tr.JSONFromYAML(&yamlbuf)
			if err != nil {
				t.Error(err)
			}
			if !cmp.Equal(jsonData, jsonbuf.Bytes()) {
				t.Errorf("json mismatch: \n%s", cmp.Diff(jsonData, jsonbuf.Bytes()))
			}
		})
		return nil
	})
}
