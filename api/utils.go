package api

import "strconv"

func parseQueryParamInt(queryValue string, defaultValue int) int {
	value, err := strconv.Atoi(queryValue)
	if err != nil {
		return defaultValue
	}

	return value
}
