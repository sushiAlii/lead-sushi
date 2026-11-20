package export

import (
	"encoding/csv"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/sushiAlii/lead-sushi/internal/leads"
)

var header = []string{
	"name", "phone", "address", "rating", "reviews", "website_status",
	"website", "weak_score", "problems", "maps_url", "place_id",
}

func Sort(rows []leads.Lead) {
	sort.SliceStable(rows, func(i, j int) bool {
		si, sj := scoreOf(rows[i]), scoreOf(rows[j])
		if si != sj {
			return si > sj
		}
		return rows[i].Reviews > rows[j].Reviews
	})
}

func Write(w io.Writer, rows []leads.Lead) error {
	writer := csv.NewWriter(w)
	if err := writer.Write(header); err != nil {
		return err
	}
	for _, row := range rows {
		if err := writer.Write(record(row)); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}

func record(l leads.Lead) []string {
	score, problems := "", ""
	if l.Audit != nil {
		score = strconv.Itoa(l.Audit.Score)
		problems = strings.Join(l.Audit.Problems, "; ")
	}
	return []string{
		l.Name,
		l.Phone,
		l.Address,
		strconv.FormatFloat(l.Rating, 'f', -1, 64),
		strconv.Itoa(l.Reviews),
		string(l.Site),
		l.Website,
		score,
		problems,
		l.MapsURL,
		l.ID,
	}
}

func scoreOf(l leads.Lead) int {
	if l.Audit == nil {
		return 0
	}
	return l.Audit.Score
}
