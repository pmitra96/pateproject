package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

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

	return 1, "pcs"
}

func main() {
	testCases := []string{
		"OrganicSetAkshayakalpaArtisanalCurdCuppc(1kg)",
		"CucumberGreenpc",
		"TomatoLocal500g",
		"Milk(2l)",
		"Rice(5kg)",
	}

	for _, tc := range testCases {
		uv, unit := parseUnitAndValue(tc)
		fmt.Printf("Input: %-50s -> unit_value: %.0f, unit: %s\n", tc, uv, unit)
	}
}
