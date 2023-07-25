package api

import (
	"strconv"
	"strings"
	"unicode"
)

func parseQueryParamInt(queryValue string, defaultValue int) int {
	value, err := strconv.Atoi(queryValue)
	if err != nil {
		return defaultValue
	}

	return value
}

func sanitizeNameFilter(input string) string {
	var sb strings.Builder

	for _, ch := range input {
		if unicode.IsLetter(ch) || unicode.IsDigit(ch) || ch == '-' {
			sb.WriteRune(ch)
		}
	}

	return sb.String()
}
