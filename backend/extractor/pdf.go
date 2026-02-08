package extractor

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/ledongthuc/pdf"
)

type ExtractedItem struct {
	Name      string  `json:"name"`
	Count     float64 `json:"count"`      // pieces from invoice (e.g. 1 unit)
	UnitValue float64 `json:"unit_value"` // size of each (e.g. 500)
	Unit      string  `json:"unit"`       // unit (e.g. g)
}

type ExtractionResult struct {
	Provider string          `json:"provider"`
	Items    []ExtractedItem `json:"items"`
}

func parseUnitAndValue(name string) (float64, string) {
	name = strings.ToLower(name)

	// First, look for parenthetical units like (1kg), (500g), (2l), etc.
	reParens := regexp.MustCompile(`\((\d+(?:\.\d+)?)\s*(g|kg|ml|l|pc|pcs)\)`)
	matchesParens := reParens.FindStringSubmatch(name)
	if len(matchesParens) > 2 {
		val, _ := strconv.ParseFloat(matchesParens[1], 64)
		unit := matchesParens[2]

		// Normalize units
		if unit == "pcs" {
			unit = "pc"
		}
		if unit == "kg" {
			return val * 1000, "g"
		}
		if unit == "l" {
			return val * 1000, "ml"
		}

		return val, unit
	}

	// Look for weight/volume with value (e.g. 500g, 1 kg, 1 l, 500 ml)
	re := regexp.MustCompile(`(\d+(?:\.\d+)?)\s*(g|kg|ml|l|pack|set|bundle)`)
	matches := re.FindStringSubmatch(name)
	if len(matches) > 2 {
		val, _ := strconv.ParseFloat(matches[1], 64)
		unit := matches[2]

		// Normalize to base units: g and ml
		if unit == "kg" {
			return val * 1000, "g"
		}
		if unit == "l" {
			return val * 1000, "ml"
		}

		return val, unit
	}

	// Look for standalone units at the end (e.g. "pc", "pcs", "kg", "g")
	reStandalone := regexp.MustCompile(`\b(\d+(?:\.\d+)?)\s*(pc|pcs|kg|g|ml|l)\b`)
	matchesStandalone := reStandalone.FindStringSubmatch(name)
	if len(matchesStandalone) > 2 {
		val, _ := strconv.ParseFloat(matchesStandalone[1], 64)
		unit := matchesStandalone[2]

		// Normalize units
		if unit == "pcs" {
			unit = "pc"
		}
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

func ParseInvoice(path string) (*ExtractionResult, error) {
	f, r, err := pdf.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var result ExtractionResult
	result.Provider = "unknown"

	var items []ExtractedItem
	totalPage := r.NumPage()

	for i := 1; i <= totalPage; i++ {
		p := r.Page(i)
		if p.V.IsNull() {
			continue
		}

		texts := p.Content().Text

		// Provider Detection
		if result.Provider == "unknown" {
			var fullPageText strings.Builder
			for _, t := range texts {
				fullPageText.WriteString(strings.ToLower(t.S))
			}
			pageText := fullPageText.String()

			if strings.Contains(pageText, "zepto") {
				result.Provider = "zepto"
			} else if strings.Contains(pageText, "blinkit") || strings.Contains(pageText, "grofers") {
				result.Provider = "blinkit"
			} else if strings.Contains(pageText, "swiggy") || strings.Contains(pageText, "instamart") {
				result.Provider = "swiggy"
			}
		}

		rows := groupTextsIntoRows(texts)

		var nameColX, qtyColX float64
		headerFound := false

		// Find header columns - handle split headers (e.g., Zepto has Description and Qty on different rows)
		for _, row := range rows {
			rowText := strings.Join(row.contents, " ")
			// Remove spaces for matching (PDF has char-by-char text)
			cleanRowText := strings.ReplaceAll(strings.ToLower(rowText), " ", "")

			// Look for Description column
			if strings.Contains(cleanRowText, "description") || strings.Contains(cleanRowText, "item") {
				if len(row.xCoords) > 0 && nameColX == 0 {
					// Find the X position where "description" or "item" text actually starts
					for i, content := range row.contents {
						lowerContent := strings.ToLower(content)
						if strings.Contains(lowerContent, "d") || strings.Contains(lowerContent, "e") ||
							strings.Contains(lowerContent, "s") || strings.Contains(lowerContent, "c") ||
							strings.Contains(lowerContent, "i") || strings.Contains(lowerContent, "t") {
							// Found start of description/item text
							nameColX = row.xCoords[i]
							break
						}
					}
					// Fallback to first X if not found
					if nameColX == 0 {
						nameColX = row.xCoords[0]
					}
				}
			}

			// Look for Qty column - find actual position of "Q" or "q"
			if strings.Contains(cleanRowText, "qty") || strings.Contains(cleanRowText, "quantity") {
				if len(row.xCoords) > 0 && qtyColX == 0 {
					// Find the X position where "qty" text actually starts
					for i, content := range row.contents {
						lowerContent := strings.ToLower(content)
						if lowerContent == "q" || strings.HasPrefix(lowerContent, "q") {
							qtyColX = row.xCoords[i]
							break
						}
					}
					// Fallback: if not found, use first X that's significantly to the right
					if qtyColX == 0 {
						for i, x := range row.xCoords {
							if x > nameColX+50 { // At least 50 units to the right
								lowerContent := strings.ToLower(row.contents[i])
								if strings.Contains(lowerContent, "q") || strings.Contains(lowerContent, "t") || strings.Contains(lowerContent, "y") {
									qtyColX = x
									break
								}
							}
						}
					}
					// Final fallback
					if qtyColX == 0 {
						qtyColX = row.xCoords[0]
					}
				}
			}
		}

		// Check if we found both columns
		if nameColX > 0 && qtyColX > 0 {
			headerFound = true
		}

		// Skip if we didn't find the columns
		if !headerFound || nameColX == 0 || qtyColX == 0 {
			continue
		}

		// Extract items - improved block detection
		var currentBlock []rowData
		pastHeaders := false

		for _, row := range rows {
			rowText := strings.Join(row.contents, " ")
			cleanRowText := strings.ReplaceAll(strings.ToLower(rowText), " ", "")

			// Check if this is a header row
			isHeaderRow := (strings.Contains(cleanRowText, "description") && strings.Contains(cleanRowText, "qty")) ||
				(strings.Contains(cleanRowText, "description") && strings.Contains(cleanRowText, "mrp")) ||
				(strings.Contains(cleanRowText, "hsn") && strings.Contains(cleanRowText, "qty"))

			if isHeaderRow {
				pastHeaders = true
				continue
			}

			// Only process rows after headers
			if !pastHeaders {
				continue
			}

			// Check for item number (1, 2, 3, etc.) at the start - indicates new item
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
					if item := extractItemFromBlock(currentBlock, nameColX, qtyColX); item != nil {
						items = append(items, *item)
					}
					currentBlock = nil
				}
				if strings.Contains(cleanRowText, "total") || strings.Contains(cleanRowText, "subtotal") {
					break
				}
			}

			// Collect rows for item extraction
			if len(row.contents) > 0 {
				currentBlock = append(currentBlock, row)
			}
		}

		// Process any remaining block
		if len(currentBlock) > 0 {
			if item := extractItemFromBlock(currentBlock, nameColX, qtyColX); item != nil {
				items = append(items, *item)
			}
		}
	}

	result.Items = items
	return &result, nil
}

func extractItemFromBlock(block []rowData, nameColX, qtyColX float64) *ExtractedItem {
	var nameParts []string
	var qty float64

	for _, row := range block {
		for i, content := range row.contents {
			x := row.xCoords[i]

			// Collect name parts (within name column range, but before qty column)
			if x >= nameColX-5 && x < qtyColX-10 {
				clean := strings.TrimSpace(content)
				// Skip numbers, percentages, prices, and rupee symbols
				if len(clean) > 0 && !strings.Contains(clean, "%") && !strings.Contains(clean, "+") &&
					!strings.Contains(clean, "0.00") && !strings.Contains(clean, "₹") {
					// Skip if it's just a number
					if _, err := strconv.ParseFloat(clean, 64); err != nil {
						nameParts = append(nameParts, clean)
					}
				}
			}

			// Look for quantity (within qty column range)
			if x >= qtyColX-5 && x < qtyColX+15 {
				q, err := parseQty(content)
				if err == nil && q > 0 && q < 100 {
					qty = q
				}
			}
		}
	}

	// Only create item if we have both name and quantity
	if len(nameParts) > 0 && qty > 0 {
		// Join with spaces (PDF extracts char-by-char or word-by-word)
		fullName := strings.Join(nameParts, " ")

		// IMPORTANT: Extract unit info from the ORIGINAL fullName BEFORE any cleaning
		// This captures patterns like (1kg), 500g, etc.
		uv, unit := parseUnitAndValue(fullName)

		// Now clean up the name (remove unit info, codes, etc)
		// Remove standalone units at the end (with word boundary or space before)
		cleanName := regexp.MustCompile(`(?i)\s+(pc|pcs|kg|g|ml|l)$`).ReplaceAllString(fullName, "")

		// Remove parenthetical units like (kg), (1kg), etc
		cleanName = regexp.MustCompile(`(?i)\([^)]*?(kg|g|ml|l|pc|pcs)[^)]*?\)`).ReplaceAllString(cleanName, "")

		// Remove patterns with numbers and units
		re := regexp.MustCompile(`(?i)(\d+\.?\d*\s*(g|kg|ml|l|pcs|pc|pack|set|bundle))|(\b\d{4,}\b)`)
		cleanName = re.ReplaceAllString(cleanName, "")

		// Replace dots with spaces
		cleanName = strings.ReplaceAll(cleanName, ".", " ")

		// Add spaces before capital letters (for camelCase words like "OrganicSet" -> "Organic Set")
		spaceBeforeCaps := regexp.MustCompile(`([a-z])([A-Z])`)
		cleanName = spaceBeforeCaps.ReplaceAllString(cleanName, "$1 $2")

		// Normalize multiple spaces to single space
		cleanName = regexp.MustCompile(`\s+`).ReplaceAllString(cleanName, " ")
		cleanName = strings.TrimSpace(cleanName)

		if len(cleanName) > 2 && len(cleanName) < 200 { // Avoid very short or very long names
			return &ExtractedItem{
				Name:      cleanName,
				Count:     qty,
				UnitValue: uv,
				Unit:      unit,
			}
		}
	}

	return nil
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

func parseQty(s string) (float64, error) {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, ",", "")
	return strconv.ParseFloat(s, 64)
}
