package api

import (
	"fmt"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/KabosuNeko/Futon/internal/models"
)

func SearchCmd(p MangaProvider, query string) tea.Cmd {
	return func() tea.Msg {
		manga, err := p.Search(query)
		return MangaSearchResultMsg{Manga: manga, Err: err}
	}
}

func GlobalSearchCmd(providers []MangaProvider, query string) tea.Cmd {
	return func() tea.Msg {
		var mu sync.Mutex
		var wg sync.WaitGroup
		var allResults []models.Manga

		for _, p := range providers {
			wg.Add(1)
			go func(provider MangaProvider) {
				defer wg.Done()
				results, err := provider.Search(query)
				if err != nil {
					return
				}
				mu.Lock()
				for i := range results {
					results[i].Title = fmt.Sprintf("%s (%s)", results[i].Title, strings.ToLower(provider.Name()))
					results[i].Provider = provider.Name()
				}
				allResults = append(allResults, results...)
				mu.Unlock()
			}(p)
		}

		wg.Wait()

		if allResults == nil {
			allResults = []models.Manga{}
		}
		return MangaSearchResultMsg{Manga: allResults, Err: nil}
	}
}

func FetchChaptersCmd(p MangaProvider, mangaID string) tea.Cmd {
	return func() tea.Msg {
		chapters, err := p.FetchChapters(mangaID)
		return ChapterListMsg{Chapters: chapters, Err: err}
	}
}

func FetchPagesCmd(p MangaProvider, chapterID string) tea.Cmd {
	return func() tea.Msg {
		urls, err := p.FetchPages(chapterID)
		return ChapterImagesMsg{URLs: urls, Err: err}
	}
}
