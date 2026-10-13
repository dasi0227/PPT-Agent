// Package decoration defines human-managed presentation decorations.
package decoration

import "fmt"

const (
	LogoWidth  = 128
	LogoHeight = 64
)

type Positions struct {
	PageNumber   string `json:"page_number"`
	DeckTitle    string `json:"deck_title"`
	SectionTitle string `json:"section_title"`
	Logo         string `json:"logo"`
}

func DefaultPositions() Positions {
	return Positions{PageNumber: "bottom-right", DeckTitle: "none", SectionTitle: "top-left", Logo: "none"}
}

func (p Positions) Validate() error {
	occupied := map[string]string{}
	for _, item := range []struct{ key, value string }{
		{"page_number", p.PageNumber}, {"deck_title", p.DeckTitle}, {"section_title", p.SectionTitle}, {"logo", p.Logo},
	} {
		if item.value == "none" && item.key != "page_number" {
			continue
		}
		switch item.value {
		case "top-left", "top-center", "top-right", "bottom-left", "bottom-center", "bottom-right":
		default:
			return fmt.Errorf("invalid %s placement %q", item.key, item.value)
		}
		if previous, exists := occupied[item.value]; exists {
			return fmt.Errorf("decoration placement %s is assigned to both %s and %s", item.value, previous, item.key)
		}
		occupied[item.value] = item.key
	}
	return nil
}

func (p Positions) Merge(updates map[string]string) (Positions, error) {
	if len(updates) == 0 {
		return p, fmt.Errorf("provide at least one decoration position")
	}
	for key, value := range updates {
		switch key {
		case "page_number":
			p.PageNumber = value
		case "deck_title":
			p.DeckTitle = value
		case "section_title":
			p.SectionTitle = value
		case "logo":
			p.Logo = value
		default:
			return p, fmt.Errorf("unknown decoration %q", key)
		}
	}
	return p, p.Validate()
}

// LogoMetadata describes the project's single current image, stored privately.
type LogoMetadata struct {
	Hash         string `json:"hash"`
	OriginalName string `json:"original_name"`
	MediaType    string `json:"media_type"`
	Extension    string `json:"extension"`
	SizeBytes    int64  `json:"size_bytes"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
}

// Image identifies verified image bytes. Display dimensions belong to Runtime.
type Image struct {
	Src  string `json:"src"`
	Hash string `json:"hash"`
}

type Context struct {
	Positions     Positions `json:"positions"`
	LogoAvailable bool      `json:"logo_available"`
	LogoSize      struct {
		Width  int `json:"width"`
		Height int `json:"height"`
	} `json:"logo_size"`
}

func ReadOnlyContext(positions Positions, logo *LogoMetadata) Context {
	c := Context{Positions: positions, LogoAvailable: logo != nil}
	c.LogoSize.Width, c.LogoSize.Height = LogoWidth, LogoHeight
	return c
}
