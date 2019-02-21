package util

import (
	"net/url"
	"strconv"
)

func GetQueryBool(query url.Values, key string) bool {
	return query.Get(key) == "true"
}

func GetQueryInt(query url.Values, key string) (int, error) {
	if val := query.Get(key); val == "" {
		return 0, strconv.ErrSyntax
	} else {
		return strconv.Atoi(val)
	}
}
