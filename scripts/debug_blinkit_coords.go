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
	f, r, err := pdf.Open("../blinkit.pdf")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	p := r.Page(1)
	texts := p.Content().Text
	rows := groupTextsIntoRows(texts)

	fmt.Printf("Total rows: %d\n\n", len(rows))

	// Find header row
	var nameColX, qtyColX float64
	for i, row := range rows {
		rowText := strings.Join(row.contents, " ")
		cleanRowText := strings.ReplaceAll(strings.ToLower(rowText), " ", "")

		if strings.Contains(cleanRowText, "description") {
			if len(row.xCoords) > 0 && nameColX == 0 {
				nameColX = row.xCoords[0]
				fmt.Printf("Row %d: Found Description column at X=%.2f\n", i, nameColX)
			}
		}

		if strings.Contains(cleanRowText, "qty") {
			if len(row.xCoords) > 0 && qtyColX == 0 {
				qtyColX = row.xCoords[0]
				fmt.Printf("Row %d: Found Qty column at X=%.2f\n", i, qtyColX)
			}
		}
	}

	fmt.Printf("\nColumn positions: Name=%.2f, Qty=%.2f\n", nameColX, qtyColX)
	fmt.Printf("Range for names: %.2f to %.2f (qtyColX-10)\n\n", nameColX-5, qtyColX-10)

	// Show first few data rows after header
	pastHeader := false
	rowCount := 0
	for i, row := range rows {
		rowText := strings.Join(row.contents, " ")
		cleanRowText := strings.ReplaceAll(strings.ToLower(rowText), " ", "")

		isHeaderRow := (strings.Contains(cleanRowText, "description") && strings.Contains(cleanRowText, "qty"))
		if isHeaderRow {
			pastHeader = true
			continue
		}

		if !pastHeader {
			continue
		}

		if strings.Contains(cleanRowText, "total") {
			break
		}

		if rowCount < 10 {
			fmt.Printf("Row %d (Y=%.2f):\n", i, row.y)
			for j, content := range row.contents {
				x := row.xCoords[j]
				inNameRange := x >= nameColX-5 && x < qtyColX-10
				inQtyRange := x >= qtyColX-5 && x < qtyColX+15
				marker := ""
				if inNameRange {
					marker = " [NAME]"
				} else if inQtyRange {
					marker = " [QTY]"
				}
				fmt.Printf("  X=%.2f: %s%s\n", x, content, marker)
			}
			fmt.Println()
			rowCount++
		}
	}
}
