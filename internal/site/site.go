package site

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
)

// Site は取得対象の1サイト。
type Site struct {
	ID   string
	Name string
	URL  string
}

var idPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// ParseSitesJSON は JSON 配列のフィード定義を解釈する。
func ParseSitesJSON(data []byte) ([]Site, error) {
	var sites []Site
	if err := json.Unmarshal(data, &sites); err != nil {
		return nil, fmt.Errorf("フィード JSON を解釈できません: %w", err)
	}
	if err := Validate(sites); err != nil {
		return nil, err
	}
	return sites, nil
}

// LoadSitesFile は JSON ファイルからフィード定義を読み込む。
func LoadSitesFile(path string) ([]Site, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseSitesJSON(data)
}

// Validate はサイト定義が安全に扱える形かを確認する。
func Validate(sites []Site) error {
	if len(sites) == 0 {
		return fmt.Errorf("サイトが登録されていません")
	}
	seen := make(map[string]struct{}, len(sites))
	for _, s := range sites {
		if !idPattern.MatchString(s.ID) || len(s.ID) > 64 {
			return fmt.Errorf("サイトIDが不正です: %q", s.ID)
		}
		if _, ok := seen[s.ID]; ok {
			return fmt.Errorf("サイトIDが重複しています: %s", s.ID)
		}
		seen[s.ID] = struct{}{}
		if s.Name == "" {
			return fmt.Errorf("サイト名が空です: %s", s.ID)
		}
		if err := validateFeedURL(s.URL); err != nil {
			return fmt.Errorf("%s: %w", s.ID, err)
		}
	}
	return nil
}

func validateFeedURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("フィードURLを解釈できません: %w", err)
	}
	scheme := strings.ToLower(u.Scheme)
	if (scheme != "http" && scheme != "https") || u.Host == "" {
		return fmt.Errorf("フィードURLは http または https にしてください: %s", raw)
	}
	return nil
}
