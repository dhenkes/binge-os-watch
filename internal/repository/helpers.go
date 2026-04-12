package repository

import (
	"strconv"
	"strings"

	"github.com/dhenkes/binge-os-watch/internal/model"
)

func buildUpdateClauses(mask []string, allowed map[string]any) ([]string, []any) {
	var sets []string
	var args []any
	for _, field := range mask {
		field = strings.TrimSpace(field)
		if val, ok := allowed[field]; ok {
			sets = append(sets, field+" = ?")
			args = append(args, val)
		}
	}
	return sets, args
}

func offsetFromToken(token string) (int, error) {
	if token == "" {
		return 0, nil
	}
	o, err := strconv.Atoi(token)
	if err != nil {
		return 0, model.NewInvalidArgument("invalid page_token")
	}
	return o, nil
}

func nextToken(offset, pageSize, total int) string {
	if offset+pageSize < total {
		return strconv.Itoa(offset + pageSize)
	}
	return ""
}
