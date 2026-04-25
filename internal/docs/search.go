package docs

import (
	"sort"
	"strings"
)

// Result is a single search hit returned by [Search].
//
// Slug identifies the document that matched. Snippet is a short excerpt
// containing the matched line plus ±2 lines of surrounding context.
// StartLine and EndLine are 1-based line numbers bounding the snippet.
// Score is an internal rank used to order results; higher is better.
type Result struct {
	// Slug is the document identifier of the matching document.
	Slug string

	// Snippet is the matched line with ±2 lines of context, joined by newlines.
	Snippet string

	// StartLine is the 1-based line number of the first line in Snippet.
	StartLine int

	// EndLine is the 1-based line number of the last line in Snippet.
	EndLine int

	// Score is the relevance rank; higher values indicate better matches.
	// Exact substring matches outrank token (word) matches.
	Score int
}

// contextLines is the number of surrounding lines included in each snippet.
const contextLines = 2

// Search performs a case-insensitive substring and token search across the
// embedded bundle, returning ranked [Result] values.
//
// The query string is matched against each line of every document in [Bundle].
// A line earns an exact-match score (2) when it contains the query as a
// substring, and a token-match score (1) when the query matches any
// space-separated word on the line. Multiple hits per document are deduplicated
// by keeping only the highest-scoring hit per document slug; callers that need
// all hits should call [SearchAll].
//
// Results are sorted descending by Score so exact matches rank first, then
// by slug for deterministic ordering within the same score tier.
//
// An empty or whitespace-only query returns nil, nil.
func Search(query string) ([]Result, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}
	return SearchAll(query, true)
}

// SearchAll is the full search implementation used by [Search].
//
// When dedup is true, only the best (highest-score) hit per slug is returned.
// When dedup is false, all hits are returned (useful for tests).
func SearchAll(query string, dedup bool) ([]Result, error) {
	lowerQuery := strings.ToLower(query)
	queryTokens := strings.Fields(lowerQuery)

	catalog, err := Catalog()
	if err != nil {
		return nil, err
	}

	// bestPerSlug tracks the top result per document when dedup=true.
	bestPerSlug := make(map[string]Result, len(catalog))
	var allResults []Result

	for _, entry := range catalog {
		data, err := ReadSlug(entry.Slug)
		if err != nil {
			return nil, err
		}
		lines := strings.Split(string(data), "\n")

		for i, line := range lines {
			lowerLine := strings.ToLower(line)
			score := scoreMatch(lowerLine, lowerQuery, queryTokens)
			if score == 0 {
				continue
			}

			start := i - contextLines
			if start < 0 {
				start = 0
			}
			end := i + contextLines
			if end >= len(lines) {
				end = len(lines) - 1
			}

			r := Result{
				Slug:      entry.Slug,
				Snippet:   strings.Join(lines[start:end+1], "\n"),
				StartLine: start + 1,
				EndLine:   end + 1,
				Score:     score,
			}

			if dedup {
				if prev, ok := bestPerSlug[entry.Slug]; !ok || score > prev.Score {
					bestPerSlug[entry.Slug] = r
				}
			} else {
				allResults = append(allResults, r)
			}
		}
	}

	var results []Result
	if dedup {
		for _, r := range bestPerSlug {
			results = append(results, r)
		}
	} else {
		results = allResults
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		return results[i].Slug < results[j].Slug
	})

	return results, nil
}

// scoreMatch returns the relevance score for a single line against a query.
//
// It returns 2 for an exact substring match, 1 for a token (word) match where
// any query token appears as a word in the line, and 0 for no match.
func scoreMatch(lowerLine, lowerQuery string, queryTokens []string) int {
	if strings.Contains(lowerLine, lowerQuery) {
		return 2
	}
	lineTokens := strings.Fields(lowerLine)
	lineSet := make(map[string]struct{}, len(lineTokens))
	for _, t := range lineTokens {
		lineSet[t] = struct{}{}
	}
	for _, qt := range queryTokens {
		if _, ok := lineSet[qt]; ok {
			return 1
		}
	}
	return 0
}
