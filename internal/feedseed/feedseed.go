package feedseed

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/ss49919201/rss-reader/internal/site"
)

// ParseSitesJSON は JSON 配列のフィード定義を解釈する。
func ParseSitesJSON(data []byte) ([]site.Site, error) {
	var sites []site.Site
	if err := json.Unmarshal(data, &sites); err != nil {
		return nil, fmt.Errorf("フィード JSON を解釈できません: %w", err)
	}
	if err := site.Validate(sites); err != nil {
		return nil, err
	}
	return sites, nil
}

// LoadSitesFile は JSON ファイルからフィード定義を読み込む。
func LoadSitesFile(path string) ([]site.Site, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseSitesJSON(data)
}
