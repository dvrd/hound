package tokenlist

import tea "github.com/charmbracelet/bubbletea"

// NewSearchResultsMsg constructs a searchResultsMsg for use in external tests.
func NewSearchResultsMsg(query string, results []SearchResult, err error) tea.Msg {
	return searchResultsMsg{query: query, results: results, err: err}
}
