package transcodefmt_test

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chanced/transcodefmt"
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

			// jsonData, err := json.Marshal(expectedYAML)
			// if err != nil {
			// 	t.Fatal(err)
			// }

			// var expectedJSON interface{}

			// err = json.Unmarshal(jsonData, &expectedJSON)
			// if err != nil {
			// 	t.Fatal(err)
			// }

			jsonData, err := transcodefmt.YAMLToJSON(yamlData)
			if err != nil {
				t.Fatal(err)
			}
			op := filepath.Join("testoutput", name+".json")
			err = os.MkdirAll(filepath.Dir(op), 0o755)
			if err != nil {
				log.Fatal(err)
			}
			err = os.WriteFile(op, jsonData, 0o644)
			if err != nil {
				log.Fatal(err)
			}
			var actualJSON interface{}
			err = json.Unmarshal(jsonData, &actualJSON)
			if err != nil {
				t.Fatal(err)
			}
			fmt.Println(string(jsonData))
			yamlFromJSON, err := transcodefmt.JSONToYAML(jsonData)
			if err != nil {
				t.Fatal(err)
			}

			var actualYAML interface{}
			err = yaml.Unmarshal(yamlFromJSON, &actualYAML)
			if err != nil {
				t.Fatal(err)
			}

			if diff := cmp.Diff(expectedYAML, actualYAML); diff != "" {
				t.Errorf("yaml mismatch:\n%s", diff)
			}
			fmt.Print("\n\n\n-------\n\n\n")
		})

		return nil
	})
}
