package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

type Rules struct {
	MovieDirectory   string `json:"movie_directory"`
	MovieFilename    string `json:"movie_filename"`
	EpisodeDirectory string `json:"episode_directory"`
	EpisodeFilename  string `json:"episode_filename"`
}

func defaultRules() Rules {
	return Rules{"{category}/{title} ({year})", "{title} ({year})", "{category}/{title} ({year})/Season {season}", "{title} - S{season}E{episode} - {episode_title}"}
}
func normalizedRules(r Rules) Rules {
	if r == (Rules{}) {
		return defaultRules()
	}
	return r
}
func validateRules(r Rules) error {
	r = normalizedRules(r)
	for i, template := range []string{r.MovieDirectory, r.MovieFilename, r.EpisodeDirectory, r.EpisodeFilename} {
		if template == "" || len(template) > 500 || strings.HasPrefix(template, "/") || strings.ContainsAny(template, `\<>:"|?*`) {
			return errors.New("规则必须是相对路径，不能包含非法字符")
		}
		allowed := []string{"{title}", "{year}"}
		if i%2 == 0 {
			allowed = append(allowed, "{category}")
		}
		if i >= 2 {
			allowed = append(allowed, "{season}", "{episode}", "{episode_title}")
		}
		stripped := template
		for _, token := range allowed {
			stripped = strings.ReplaceAll(stripped, token, "值")
		}
		if strings.ContainsAny(stripped, "{}") || (i%2 == 1 && strings.Contains(stripped, "/")) {
			return errors.New("规则包含未知变量，文件名不能包含目录分隔符")
		}
		for _, part := range strings.Split(stripped, "/") {
			if part == "" || part == "." || part == ".." || strings.TrimSpace(part) != part || invalidName.MatchString(part) {
				return errors.New("规则不能包含空目录、路径跳转或非法字符")
			}
		}
		if !strings.Contains(template, "{title}") {
			return errors.New("每个规则都必须包含 {title}")
		}
	}
	return nil
}
func ruleTarget(m Media, root string, rules Rules) (string, error) {
	rules = normalizedRules(rules)
	if e := validateRules(rules); e != nil {
		return "", e
	}
	title, e := safeName(m.Title)
	if e != nil {
		return "", e
	}
	dir, file := rules.MovieDirectory, rules.MovieFilename
	category := "电影"
	if m.Kind != "movie" {
		if m.Kind != "episode" || m.Season < 0 || m.Season > 999 || m.Episode < 1 || m.Episode > 99999 {
			return "", errors.New("季集信息不完整，需先在飞牛影视校正")
		}
		dir, file = rules.EpisodeDirectory, rules.EpisodeFilename
		category = "电视剧"
	}
	if strings.Contains(dir+file, "{year}") && !yearPattern.MatchString(m.Year) {
		return "", errors.New("规则需要年份，请先在飞牛影视校正识别信息")
	}
	rootName := filepath.Base(filepath.Clean(root))
	if (m.Kind == "movie" && rootName == "电影") || (m.Kind == "episode" && (rootName == "电视剧" || rootName == "剧集" || rootName == "综艺")) {
		category = ""
	}
	episodeTitle := ""
	if strings.Contains(dir+file, "{episode_title}") {
		if strings.TrimSpace(m.EpisodeTitle) != "" {
			episodeTitle, e = safeName(m.EpisodeTitle)
			if e != nil {
				return "", e
			}
		} else if file == defaultRules().EpisodeFilename && !strings.Contains(dir, "{episode_title}") {
			file = "{title} - S{season}E{episode}"
		} else {
			return "", errors.New("规则需要单集名称，请先在飞牛影视补全识别信息")
		}
	}
	replacer := strings.NewReplacer("{title}", title, "{year}", m.Year, "{category}", category, "{season}", fmt.Sprintf("%02d", m.Season), "{episode}", fmt.Sprintf("%02d", m.Episode), "{episode_title}", episodeTitle)
	parts := []string{root}
	for _, part := range strings.Split(dir, "/") {
		expanded := replacer.Replace(part)
		if expanded == "" && part == "{category}" {
			continue
		}
		clean, e := safeName(expanded)
		if e != nil {
			return "", e
		}
		parts = append(parts, clean)
	}
	filename, e := safeName(replacer.Replace(file))
	if e != nil {
		return "", e
	}
	parts = append(parts, filename+filepath.Ext(m.Source))
	target := filepath.Join(parts...)
	if !inside(root, target) {
		return "", errors.New("规则目标超出影视库目录")
	}
	return target, nil
}
