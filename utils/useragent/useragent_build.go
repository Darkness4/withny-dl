//go:build build

package main

import (
	_ "embed"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"text/template"

	"github.com/Masterminds/sprig/v3"
)

//go:embed useragent.go.tpl
var templ string

func onlyNumber(s string) int {
	res := ""
	for _, c := range s {
		if c >= '0' && c <= '9' {
			res += string(c)
		} else {
			break
		}
	}
	i, _ := strconv.Atoi(res)
	return i
}

func findFirefoxLatestVersion() int {
	res, err := http.Get("https://www.browsers.fyi/api/")
	if err != nil {
		panic(err)
	}
	defer res.Body.Close()
	var v struct {
		Firefox struct {
			EngineVersion string `json:"engine_version"`
		} `json:"firefox"`
	}
	if err := json.NewDecoder(res.Body).Decode(&v); err != nil {
		panic(err)
	}
	return onlyNumber(v.Firefox.EngineVersion)
}

func main() {
	f, err := os.OpenFile("useragent.go", os.O_TRUNC|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	template.Must(template.New("base").Funcs(sprig.TxtFuncMap()).Parse(templ)).
		Execute(f, map[string]any{
			"FirefoxRevision": findFirefoxLatestVersion(),
			"OSXVersion":      "10.15",
			"NTVersion":       "10.0",
		})
}
