package main

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/ledongthuc/pdf"
)

type ExtractedItem struct {
	Name      string  `json:"name"`
	Count     float64 `json:"count"`
	UnitValue float64 `json:"unit_value"`
	Unit      string  `json:"unit"`
}

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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func parseQty(s string) (float64, error) {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, ",", "")
	return strconv.ParseFloat(s, 64)
}

func parseUnitAndValue(name string) (float64, string) {
	name = strings.ToLower(name)
	re := regexp.MustCompile(`(\d+(\.\d+)?)\s*(g|kg|ml|l|pack|set|bundle)`)
	matches := re.FindStringSubmatch(name)
	if len(matches) > 3 {
		val, _ := strconv.ParseFloat(matches[1], 64)
		unit := matches[3]

		// Normalize to base units
		if unit == "kg" {
			return val * 1000, "g"
		}
		if unit == "l" {
			return val * 1000, "ml"
		}

		return val, unit
	}

	return 1, "pcs"
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run extract_zepto.go <pdf_file>")
		os.Exit(1)
	}

	pdfPath := os.Args[1]

	f, r, err := pdf.Open(pdfPath)
	if err != nil {
		fmt.Printf("Error opening PDF: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	fmt.Printf("📄 Processing: %s\n", pdfPath)
	fmt.Printf("📖 Pages: %d\n\n", r.NumPage())

	var allItems []ExtractedItem

	for pageNum := 1; pageNum <= r.NumPage(); pageNum++ {
		fmt.Printf("--- Page %d ---\n", pageNum)

		p := r.Page(pageNum)
		if p.V.IsNull() {
			continue
		}

		texts := p.Content().Text
		rows := groupTextsIntoRows(texts)

		fmt.Printf("Total rows: %d\n", len(rows))

		// Find column positions
		var nameColX, qtyColX float64

		for _, row := range rows {
			rowText := strings.Join(row.contents, " ")
			cleanRowText := strings.ReplaceAll(strings.ToLower(rowText), " ", "")

			if strings.Contains(cleanRowText, "description") && nameColX == 0 {
				nameColX = row.xCoords[0]
				fmt.Printf("Found Description column at X=%.2f\n", nameColX)
			}

			if strings.Contains(cleanRowText, "qty") && qtyColX == 0 {
				qtyColX = row.xCoords[0]
				fmt.Printf("Found Qty column at X=%.2f\n", qtyColX)
			}
		}

		if nameColX == 0 || qtyColX == 0 {
			fmt.Println("⚠️  Could not find column headers, skipping page")
			continue
		}

		fmt.Printf("\n🔍 Extracting items...\n\n")

		// Extract items
		var currentBlock []rowData
		blockNum := 0
		pastHeaders := false

		for i, row := range rows {
			rowText := strings.Join(row.contents, " ")
			cleanRowText := strings.ReplaceAll(strings.ToLower(rowText), " ", "")

			// Check if this is a header row
			isHeaderRow := (strings.Contains(cleanRowText, "description") && strings.Contains(cleanRowText, "qty")) ||
				(strings.Contains(cleanRowText, "description") && strings.Contains(cleanRowText, "mrp")) ||
				(strings.Contains(cleanRowText, "hsn") && strings.Contains(cleanRowText, "qty"))

			if isHeaderRow {
				fmt.Printf("Row %d: [HEADER] Found header row, starting item collection\n", i)
				pastHeaders = true
				continue
			}

			// Only process rows after headers
			if !pastHeaders {
				continue
			}

			// Check for item number (1, 2, 3, etc.) at the start
			isItemStart := false
			if len(row.contents) > 0 {
				firstContent := strings.TrimSpace(row.contents[0])
				if num, err := strconv.Atoi(firstContent); err == nil && num > 0 && num < 100 {
					isItemStart = true
				}
			}

			// End block on totals or new item start
			if strings.Contains(cleanRowText, "total") || strings.Contains(cleanRowText, "subtotal") || (isItemStart && len(currentBlock) > 0) {
				if len(currentBlock) > 0 {
					blockNum++
					fmt.Printf("\n--- Block %d (%d rows) ---\n", blockNum, len(currentBlock))

					// Debug: show first few rows of block
					for bi, blockRow := range currentBlock {
						if bi < 3 {
							rowText := strings.Join(blockRow.contents, " ")
							cleanText := strings.ReplaceAll(rowText, " ", "")
							fmt.Printf("  Row %d: %s\n", bi, cleanText[:min(80, len(cleanText))])
						}
					}

					// Extract item from block
					var nameParts []string
					var qty float64

					for _, blockRow := range currentBlock {
						for j, content := range blockRow.contents {
							x := blockRow.xCoords[j]

							// Collect name parts
							if x >= nameColX-5 && x < qtyColX-10 {
								clean := strings.TrimSpace(content)
								// Skip numbers, percentages, prices
								if len(clean) > 0 && !strings.Contains(clean, "%") && !strings.Contains(clean, "+") && !strings.Contains(clean, "0.00") && !strings.Contains(clean, "₹") {
									// Skip if it's just a number
									if _, err := strconv.ParseFloat(clean, 64); err != nil {
										nameParts = append(nameParts, clean)
									}
								}
							}

							// Look for quantity
							if x >= qtyColX-5 && x < qtyColX+15 {
								q, err := parseQty(content)
								if err == nil && q > 0 && q < 100 {
									qty = q
									fmt.Printf("  Found qty=%.0f at X=%.2f\n", q, x)
								}
							}
						}
					}

					fmt.Printf("  Collected: %d name parts, qty=%.0f\n", len(nameParts), qty)

					if len(nameParts) > 0 && qty > 0 {
						// Join without spaces since each part is a single character
						fullName := strings.Join(nameParts, "")

						// Clean up name
						re := regexp.MustCompile(`(\(\d+.*?\))|(\d+\.?\d*\s*(g|kg|ml|l|pcs|pc|pack|set|bundle))|(\b\d{4,}\b)`)
						cleanName := re.ReplaceAllString(fullName, "")
						cleanName = strings.TrimSpace(cleanName)

						// Remove dots and extra spaces
						cleanName = strings.ReplaceAll(cleanName, ".", " ")
						cleanName = regexp.MustCompile(`\s+`).ReplaceAllString(cleanName, " ")
						cleanName = strings.TrimSpace(cleanName)

						if len(cleanName) > 2 && len(cleanName) < 200 {
							uv, unit := parseUnitAndValue(fullName)

							item := ExtractedItem{
								Name:      cleanName,
								Count:     qty,
								UnitValue: uv,
								Unit:      unit,
							}

							allItems = append(allItems, item)

							fmt.Printf("✅ Item: %s\n", cleanName)
							fmt.Printf("   Qty: %.0f, Unit: %.0f %s\n", qty, uv, unit)
						} else {
							fmt.Printf("⚠️  Rejected: '%s' (len=%d)\n", cleanName, len(cleanName))
						}
					} else {
						fmt.Printf("⚠️  No valid item (nameParts=%d, qty=%.0f)\n", len(nameParts), qty)
					}

					currentBlock = nil
				}

				if strings.Contains(cleanRowText, "total") || strings.Contains(cleanRowText, "subtotal") {
					fmt.Printf("Row %d: [TOTAL] End of items\n", i)
					break
				}
			}

			// Add to current block
			if len(row.contents) > 0 {
				currentBlock = append(currentBlock, row)
			}
		}

		fmt.Println()
	}

	fmt.Printf("\n📦 Total items extracted: %d\n\n", len(allItems))

	// Output JSON
	jsonData, _ := json.MarshalIndent(allItems, "", "  ")
	fmt.Println(string(jsonData))
}
