package export

import (
	"bytes"
	"encoding/csv"
	"strings"
	"testing"

	"github.com/sushiAlii/lead-sushi/internal/audit"
	"github.com/sushiAlii/lead-sushi/internal/leads"
	"github.com/sushiAlii/lead-sushi/internal/places"
)

func lead(name string, reviews int, score *int, problems ...string) leads.Lead {
	l := leads.Lead{Place: places.Place{ID: "id-" + name, Name: name, Reviews: reviews, Rating: 4.5}, Site: leads.SiteOwn}
	if score != nil {
		l.Audit = &audit.Result{Score: *score, Problems: problems}
	}
	return l
}

func intp(n int) *int { return &n }

func TestSortOrdersByScoreThenReviews(t *testing.T) {
	rows := []leads.Lead{
		lead("low-score", 500, intp(5)),
		lead("high-score-few-reviews", 10, intp(9)),
		lead("high-score-many-reviews", 80, intp(9)),
	}

	Sort(rows)

	got := []string{rows[0].Name, rows[1].Name, rows[2].Name}
	want := []string{"high-score-many-reviews", "high-score-few-reviews", "low-score"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %v, want %v", got, want)
		}
	}
}

func TestSortWithoutAuditFallsBackToReviews(t *testing.T) {
	rows := []leads.Lead{lead("few", 5, nil), lead("many", 50, nil)}

	Sort(rows)

	if rows[0].Name != "many" {
		t.Errorf("first = %q, want many", rows[0].Name)
	}
}

func TestWriteProducesHeaderAndRows(t *testing.T) {
	rows := []leads.Lead{lead("Smile Dental", 40, intp(8), "no-viewport", "slow")}
	var buf bytes.Buffer

	if err := Write(&buf, rows); err != nil {
		t.Fatal(err)
	}

	records, err := csv.NewReader(strings.NewReader(buf.String())).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 {
		t.Fatalf("records = %d, want header + 1 row", len(records))
	}
	if strings.Join(records[0], ",") != strings.Join(header, ",") {
		t.Errorf("header = %v", records[0])
	}
	row := records[1]
	if row[0] != "Smile Dental" || row[4] != "40" || row[5] != "has_site" || row[7] != "8" || row[8] != "no-viewport; slow" {
		t.Errorf("row = %v", row)
	}
}

func TestWriteLeavesScoreBlankWithoutAudit(t *testing.T) {
	var buf bytes.Buffer

	if err := Write(&buf, []leads.Lead{lead("No Audit", 10, nil)}); err != nil {
		t.Fatal(err)
	}

	records, _ := csv.NewReader(&buf).ReadAll()
	if records[1][7] != "" || records[1][8] != "" {
		t.Errorf("score/problems = %q/%q, want blank", records[1][7], records[1][8])
	}
}
