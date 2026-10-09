package id

import (
	"fmt"
	"strings"
)

// IDPrefix is the type-specific part of an ID before the underscore, e.g. "us" in "us_0f3k2m9a1b7c". An app composes its prefixes from a vocabulary of two-letter words, one per concept, so every prefix reads unambiguously: "apke" is API key.
type IDPrefix string

// ComposePrefix concatenates two-letter vocabulary words into a prefix. It panics on a word that is not two lowercase ASCII letters, since prefixes are composed once at package initialization and a malformed one would mint unreadable IDs.
func ComposePrefix(words ...string) IDPrefix {
	if len(words) == 0 {
		panic("id: ComposePrefix needs at least one word")
	}
	for _, w := range words {
		if !isVocabularyWord(w) {
			panic(fmt.Sprintf("id: vocabulary word %q must be two lowercase letters", w))
		}
	}
	return IDPrefix(strings.Join(words, ""))
}

// IsVocabularyWord reports whether w is two lowercase ASCII letters.
func IsVocabularyWord(w string) bool {
	return isVocabularyWord(w)
}

func isVocabularyWord(w string) bool {
	return len(w) == 2 && w[0] >= 'a' && w[0] <= 'z' && w[1] >= 'a' && w[1] <= 'z'
}
