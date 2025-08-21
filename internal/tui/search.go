package tui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func (m *Model) performSearch() {
	query := m.textInput.Value()

	if query == "" {
		m.searchResults = []searchResult{}
		m.currentSearchResultIdx = -1
		return
	}
	m.searchResults = []searchResult{}
	m.currentSearchResultIdx = -1

	lowerQuery := strings.ToLower(query)

	for colIdx, col := range m.displayColumns {
		for cardIdx, card := range col.Cards {
			titleMatch := strings.Contains(strings.ToLower(card.Title), lowerQuery)
			contentMatch := strings.Contains(strings.ToLower(card.Content), lowerQuery)
			if titleMatch || contentMatch {
				m.searchResults = append(m.searchResults, searchResult{
					colIndex:  colIdx,
					cardIndex: cardIdx + 1,
				})
			}
		}
	}
}

func (m *Model) jumpToFirstResult(showMessageOnFail bool) tea.Cmd {
	if len(m.searchResults) == 0 {
		if showMessageOnFail {
			m.statusMessage = "Pattern not found: " + m.lastSearchQuery
			m.textInput.SetValue("")
			return clearStatusCmd(2 * time.Second)
		}
		return nil
	}

	currentCol := m.focusedColumn
	currentCard := m.currentFocusedCard()

	if m.lastSearchDirection == "/" {
		nextIdx := -1
		for i, res := range m.searchResults {
			if res.colIndex > currentCol || (res.colIndex == currentCol && res.cardIndex > currentCard) {
				nextIdx = i
				break
			}
		}
		if nextIdx == -1 { // Wrap around
			nextIdx = 0
		}
		m.currentSearchResultIdx = nextIdx
		m.jumpTo(m.searchResults[nextIdx])
	} else { // "?"
		nextIdx := -1
		for i := len(m.searchResults) - 1; i >= 0; i-- {
			res := m.searchResults[i]
			if res.colIndex < currentCol || (res.colIndex == currentCol && res.cardIndex < currentCard) {
				nextIdx = i
				break
			}
		}
		if nextIdx == -1 { // Wrap around
			nextIdx = len(m.searchResults) - 1
		}
		m.currentSearchResultIdx = nextIdx
		m.jumpTo(m.searchResults[nextIdx])
	}
	return nil
}

func (m *Model) findNext() tea.Cmd {
	if m.lastSearchQuery == "" {
		m.statusMessage = "No previous search"
		return clearStatusCmd(2 * time.Second)
	}
	if len(m.searchResults) == 0 {
		m.performSearch()
		if len(m.searchResults) == 0 {
			m.statusMessage = "Pattern not found: " + m.lastSearchQuery
			return clearStatusCmd(2 * time.Second)
		}
	}

	if m.lastSearchDirection == "/" {
		m.currentSearchResultIdx = (m.currentSearchResultIdx + 1) % len(m.searchResults)
	} else { // "?"
		m.currentSearchResultIdx--
		if m.currentSearchResultIdx < 0 {
			m.currentSearchResultIdx = len(m.searchResults) - 1
		}
	}
	res := m.searchResults[m.currentSearchResultIdx]
	m.jumpTo(res)
	return nil
}

func (m *Model) findPrev() tea.Cmd {
	if m.lastSearchQuery == "" {
		m.statusMessage = "No previous search"
		return clearStatusCmd(2 * time.Second)
	}
	if len(m.searchResults) == 0 {
		m.performSearch()
		if len(m.searchResults) == 0 {
			m.statusMessage = "Pattern not found: " + m.lastSearchQuery
			return clearStatusCmd(2 * time.Second)
		}
	}

	if m.lastSearchDirection == "/" {
		m.currentSearchResultIdx--
		if m.currentSearchResultIdx < 0 {
			m.currentSearchResultIdx = len(m.searchResults) - 1
		}
	} else { // "?"
		m.currentSearchResultIdx = (m.currentSearchResultIdx + 1) % len(m.searchResults)
	}
	res := m.searchResults[m.currentSearchResultIdx]
	m.jumpTo(res)
	return nil
}
