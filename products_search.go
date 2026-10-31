package main

import "unicode/utf8"

// minIndexedSearchRunes is the shortest term the products_search trigram index
// can answer; anything shorter uses the plain LIKE queries.
const minIndexedSearchRunes = 3

func searchUsesIndex(search string) bool {
	return utf8.RuneCountInString(search) >= minIndexedSearchRunes
}
