package places

import (
	"fmt"
	"sort"
)

type City struct {
	Name  string
	State string
	Areas []string
}

var Cities = map[string]City{
	"sydney": {
		Name:  "Sydney",
		State: "NSW",
		Areas: []string{
			"Parramatta", "Bondi", "Manly", "Penrith", "Liverpool",
			"Chatswood", "Hornsby", "Sutherland", "Blacktown", "Randwick",
		},
	},
	"melbourne": {
		Name:  "Melbourne",
		State: "VIC",
		Areas: []string{
			"Richmond", "St Kilda", "Footscray", "Brunswick", "Box Hill",
			"Dandenong", "Frankston", "Werribee", "Preston", "Brighton",
		},
	},
	"brisbane": {
		Name:  "Brisbane",
		State: "QLD",
		Areas: []string{
			"Fortitude Valley", "Indooroopilly", "Chermside", "Carindale", "Sunnybank",
			"Redcliffe", "Ipswich", "Logan", "Wynnum", "Mitchelton",
		},
	},
	"perth": {
		Name:  "Perth",
		State: "WA",
		Areas: []string{
			"Fremantle", "Joondalup", "Subiaco", "Armadale", "Rockingham",
			"Midland", "Cannington", "Morley", "Mandurah", "Scarborough",
		},
	},
}

var Niches = map[string][]string{
	"clinics": {"dentist", "physiotherapist", "chiropractor", "podiatrist"},
	"beauty":  {"beauty salon", "cosmetic clinic", "nail salon"},
	"trades": {
		"plumber", "electrician", "house cleaner", "landscaper",
		"roofer", "painter", "handyman", "pest control",
	},
	"auto": {"mechanic", "mobile mechanic", "auto electrician"},
	"pro":  {"accountant", "lawyer", "mortgage broker"},
}

func CityNames() []string {
	return sortedKeys(Cities)
}

func NicheNames() []string {
	return sortedKeys(Niches)
}

func Queries(city City, terms []string) []string {
	queries := make([]string, 0, len(city.Areas)*len(terms))
	for _, area := range city.Areas {
		for _, term := range terms {
			queries = append(queries, fmt.Sprintf("%s in %s, %s %s", term, area, city.Name, city.State))
		}
	}
	return queries
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
