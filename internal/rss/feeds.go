package rss

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

//go:embed feeds.json
var embeddedFeeds []byte

// LoadEmbeddedSites はバイナリに埋め込んだ feeds.json からサイト一覧を読み込む。
func LoadEmbeddedSites() ([]Site, error) {
	var sites []Site
	if err := json.Unmarshal(embeddedFeeds, &sites); err != nil {
		return nil, fmt.Errorf("feeds.json を解釈できません: %w", err)
	}
	if err := Validate(sites); err != nil {
		return nil, err
	}
	return sites, nil
}
