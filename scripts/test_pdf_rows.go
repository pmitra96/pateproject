package main

import (
	"fmt"
	"strings"

	"github.com/ledongthuc/pdf"
)

type rowData struct {
	y        float64
	contents []string
	xCoords  []float64
}

func groupTextsIntoRows(texts []pdf.Text) []rowData {
	if len(texts) == 0 {
		return nil
	}

	var rows []rowData
	tolerance := 2.0

	for _, t := range texts {
		content := strings.TrimSpace(t.S)
		if content == "" {
			continue
		}

		placed := false
		for i := range rows {
			if abs(rows[i].y-t.Y) < tolerance {
				rows[i].contents = append(rows[i].contents, content)
				rows[i].xCoords = append(rows[i].xCoords, t.X)
				placed = true
				break
			}
		}

		if !placed {
			rows = append(rows, rowData{
				y:        t.Y,
				contents: []string{content},
				xCoords:  []float64{t.X},
			})
		}
	}

	return rows
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func main() {
	f, r, err := pdf.Open("../zepto.pdf")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	p := r.Page(1)
	texts := p.Content().Text
	rows := groupTextsIntoRows(texts)

	fmt.Printf("Total rows: %d\n\n", len(rows))

	for i, row := range rows {
		rowText := strings.Join(row.contents, " ")
		cleanRowText := strings.ReplaceAll(strings.ToLower(rowText), " ", "")

		if strings.Contains(cleanRowText, "description") || strings.Contains(cleanRowText, "qty") {
			fmt.Printf("Row %d (Y=%.2f):\n", i, row.y)
			fmt.Printf("  Raw: %s\n", rowText)
			fmt.Printf("  Clean: %s\n", cleanRowText)
			fmt.Printf("  Has 'description': %v\n", strings.Contains(cleanRowText, "description"))
			fmt.Printf("  Has 'qty': %v\n\n", strings.Contains(cleanRowText, "qty"))
		}
	}
}
