package main

import (
	"encoding/json"
	"encoding/xml"
	"net/url"
	"os"
	"sort"
)

type SaveFile struct {
	XMLName xml.Name     `xml:"map"`
	Ints    []SaveInt    `xml:"int"`
	Strings []SaveString `xml:"string"`
}

type SaveInt struct {
	XMLName xml.Name `xml:"int"`
	Name    string   `xml:"name,attr"`
	Value   int      `xml:"value,attr"`
}

type SaveString struct {
	XMLName xml.Name `xml:"string"`
	Name    string   `xml:"name,attr"`
	Value   string   `xml:",chardata"`
}

func main() {
	f, err := os.Open("/data/data/com.okidokico.okgolf/shared_prefs/ com.okidokico.okgolf.v2.playerprefs.xml")
	if err != nil {
		panic(err)
	}
	dec := xml.NewDecoder(f)
	data := SaveFile{}
	dec.Decode(&data)

	for i := range data.Ints {
		data.Ints[i].Name, _ = url.QueryUnescape(data.Ints[i].Name)
	}
	for i := range data.Strings {
		data.Strings[i].Name, _ = url.QueryUnescape(data.Strings[i].Name)
		data.Strings[i].Value, _ = url.QueryUnescape(data.Strings[i].Value)
	}

	sort.Slice(data.Ints, func(i, j int) bool { return data.Ints[i].Name < data.Ints[j].Name })
	sort.Slice(data.Strings, func(i, j int) bool { return data.Strings[i].Name < data.Strings[j].Name })

	out := make(map[string]any)
	for _, v := range data.Ints {
		out[v.Name] = v.Value
	}
	for _, v := range data.Strings {
		out[v.Name] = v.Value
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(&out)
}
