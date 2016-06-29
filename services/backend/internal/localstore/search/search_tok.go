package search

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/fatih/camelcase"

	"sourcegraph.com/sourcegraph/srclib/graph"
)

var delims = regexp.MustCompile(`[/.:\$\(\)\*\%\#\@\[\]\{\}]+`)

// TSVector encapsulates the tokens and token counts that represent the tsvector
// representation of a def. Word proximity matters for tsquery queries that use
// the '&' operator, so we keep track of the order in which tokens are added.
type TSVector struct {
	A     map[string]int // Most important lexemes (e.g., definition name, definition name camelCase tokens)
	AToks []string

	B     map[string]int // 2nd most important lexemes (e.g., non-name def path components, repository path, unit name)
	BToks []string

	C     map[string]int // 3rd most important lexemes (e.g., file parts, camelCase-tokenized def path components)
	CToks []string

	D     map[string]int // 4th most important lexemes (other)
	DToks []string
}

// Add adds a word to the TSVector with specified count and class (A, B, C, or D)
func (v *TSVector) Add(class string, word string, count int) {
	var (
		m map[string]int
		s *[]string
	)

	switch class {
	case "A":
		if v.A == nil {
			v.A = make(map[string]int)
		}
		m, s = v.A, &v.AToks
	case "B":
		if v.B == nil {
			v.B = make(map[string]int)
		}
		m, s = v.B, &v.BToks
	case "C":
		if v.C == nil {
			v.C = make(map[string]int)
		}
		m, s = v.C, &v.CToks
	case "D":
		if v.D == nil {
			v.D = make(map[string]int)
		}
		m, s = v.D, &v.DToks
	default:
		panic(fmt.Sprintf(`cannot set weight for TSVector to unrecognized class %q; choices are "A", "B", "C", and "D"`, class))
	}
	if _, exists := m[word]; !exists {
		*s = append(*s, word)
	}
	m[word] += count
}

func ToTSVector(def *graph.Def) *TSVector {
	tsvector := TSVector{}

	repoParts := strings.Split(def.Repo, "/")
	if len(repoParts) >= 1 && (strings.HasSuffix(repoParts[0], ".com") || strings.HasSuffix(repoParts[0], ".org")) {
		repoParts = repoParts[1:]
	}
	for _, w := range repoParts {
		tsvector.Add("B", w, 1)
	}
	tsvector.Add("B", repoParts[len(repoParts)-1], 2) // the last path component tends to be the repository name

	unitParts := strings.Split(def.Unit, "/")
	for _, w := range unitParts {
		tsvector.Add("B", w, 1)
	}
	tsvector.Add("B", unitParts[len(unitParts)-1], 2)

	defParts := delims.Split(def.Path, -1)
	for _, w := range defParts {
		tsvector.Add("B", w, 2)
	}
	lastDefPart := defParts[len(defParts)-1]
	tsvector.Add("A", lastDefPart, 3) // mega extra points for matching the last component of the def path (typically the "name" of the def)
	for _, w := range splitCaseWords(lastDefPart) {
		tsvector.Add("A", w, 1) // more points for matching last component of def path
	}
	// CamelCase and snake_case tokens in the definition path
	for _, part := range defParts {
		for _, w := range splitCaseWords(part) {
			tsvector.Add("C", w, 1)
		}
	}

	fileParts := strings.Split(filepath.ToSlash(def.File), "/")
	for _, w := range fileParts {
		tsvector.Add("C", w, 1)
	}
	tsvector.Add("C", fileParts[len(fileParts)-1], 2)

	tsvector.Add("A", def.Name, 1)

	return &tsvector
}

func splitCaseWords(w string) []string {
	if strings.Contains(w, "_") {
		return strings.Split(w, "_")
	}
	return camelcase.Split(w)
}

func UserQueryToksToTSQuery(toks []string) string {
	if len(toks) == 0 {
		return ""
	}
	tokMatch := strings.Join(toks, " & ")
	return tokMatch
}

func BagOfWordsToTokens(words []string, wordCounts map[string]int) []string {
	var v []string
	for _, word := range words {
		if word == "" {
			continue
		}
		count := wordCounts[word]
		for i := 0; i < count; i++ {
			v = append(v, word)
		}
	}
	return v
}
