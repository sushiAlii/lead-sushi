package places

import "testing"

func TestQueriesLoopAreasOutsideTermsInside(t *testing.T) {
	city := City{Name: "Sydney", State: "NSW", Areas: []string{"Manly", "Bondi"}}

	got := Queries(city, []string{"dentist", "physio"})

	want := []string{
		"dentist in Manly, Sydney NSW",
		"physio in Manly, Sydney NSW",
		"dentist in Bondi, Sydney NSW",
		"physio in Bondi, Sydney NSW",
	}
	if len(got) != len(want) {
		t.Fatalf("got %d queries, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("query %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestPresetsAreComplete(t *testing.T) {
	for name, city := range Cities {
		if city.Name == "" || city.State == "" || len(city.Areas) == 0 {
			t.Errorf("city %q is incomplete: %+v", name, city)
		}
	}
	for name, terms := range Niches {
		if len(terms) == 0 {
			t.Errorf("niche %q has no terms", name)
		}
	}
}

func TestNamesAreSorted(t *testing.T) {
	names := CityNames()
	for i := 1; i < len(names); i++ {
		if names[i-1] > names[i] {
			t.Fatalf("city names not sorted: %v", names)
		}
	}
}
