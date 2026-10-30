package api

import (
	"net/url"
	"strings"
	"unicode/utf8"
)

// Input limits. The numeric ones keep values inside their database columns
// (price NUMERIC(12,2), stock and quantity INTEGER), so out-of-range input
// gets a 400 instead of a database error.
const (
	maxNameLen        = 200
	maxDescriptionLen = 5000
	maxEmailLen       = 254
	maxAddressLen     = 500
	maxURLLen         = 2048
	maxPrice          = 1_000_000_000
	maxStock          = 1_000_000_000
	maxCartQuantity   = 1000 // of one product in a cart
)

func tooLong(s string, max int) bool {
	return utf8.RuneCountInString(s) > max
}

// validImageURL accepts absolute http(s) URLs only, so links such as
// "javascript:..." can never end up in the frontend.
func validImageURL(s string) bool {
	if len(s) > maxURLLen {
		return false
	}
	u, err := url.Parse(s)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

// likeEscaper makes %, _ and \ match literally in a LIKE/ILIKE pattern.
var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
