//go:build ignore
// +build ignore

package main

import (
	"bytes"
	"go/format"
	"io/ioutil"
	"os"
	"strings"
)

var packageName = "package pointer"

func main() {
	f, err := os.Open("template.txt")
	if err != nil {
		panic(err)
	}
	fileData, err := ioutil.ReadAll(f)
	if err != nil {
		panic(err)
	}

	w := new(bytes.Buffer)
	start_pos := strings.Index(string(fileData), packageName)
	w.WriteString(string(fileData)[start_pos : start_pos+len(packageName)])

	ts := []string{"Byte", "Complex64", "Complex128", "Float32", "Float64", "Int", "Int8", "Int16", "Int32", "Int64", "Rune", "Uint", "Uint8", "Uint16", "Uint32", "Uint64", "Uintptr"}

	for _, upper := range ts {
		lower := strings.ToLower(upper)
		data := string(fileData)

		data = data[start_pos+len(packageName):]

		data = strings.Replace(data, "{{upper}}", upper, -1)
		data = strings.Replace(data, "{{lower}}", lower, -1)

		w.WriteString(data)
		w.WriteString("\r\n")
	}

	out, err := format.Source(w.Bytes())
	if err != nil {
		panic(err)
	}
	if err := ioutil.WriteFile("types.go", out, 0660); err != nil {
		panic(err)
	}
}
