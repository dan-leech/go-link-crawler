package utils

import (
	"fmt"
	"go-link-crawler/log"
	"net/url"
	"regexp"
	"strings"
)

var (
	reDomain = regexp.MustCompile(`(?im)https?://(?:www.)?(.+?)(?:/|$)`)
	reScheme = regexp.MustCompile(`(?im)^([a-z-_.]+):`)
)

func IsInnerUrl(src string, baseUrl *url.URL) bool {
	src = strings.TrimLeft(src, "www.")
	matches := reDomain.FindAllStringSubmatch(src, -1)
	if len(matches) > 0 && matches[0][1] == baseUrl.Host {
		return true
	}
	return false
}

func GetUrlScheme(src string) string {
	matches := reScheme.FindAllStringSubmatch(src, -1)
	if len(matches) > 0 {
		return matches[0][1]
	}
	return ""
}

func RelativeUrlToFull(src, curUrl string, baseUrl *url.URL) string {
	src = strings.TrimSpace(src)
	if !strings.HasPrefix(src, "http://") && !strings.HasPrefix(src, "https://") {
		if strings.HasPrefix(src, "//") {
			src = baseUrl.Scheme + ":" + src
		} else if strings.HasPrefix(src, "/") { // absolute links
			src = src[1:]
			src = baseUrl.Scheme + "://" + baseUrl.Host + "/" + src
		} else { // relative links
			cu, err := url.Parse(curUrl)
			if err != nil {
				log.WithTrace("utils", "RelativeUrlToFull").Errorf("url.Parse('%s') err: %v", curUrl, err)
				return ""
			}
			path := strings.Split(cu.Path, "/")
			// clean up scripts to resolve relative path
			if strings.Contains(path[len(path)-1], ".") {
				path = path[:len(path)-1]
			}
			// clean empty elements
			i := 0
			for _, p := range path {
				if p != "" {
					path[i] = p
					i++
				}
			}
			path = path[:i]

			path = append([]string{cu.Host}, path...)
			path = append(path, src)
			src = fmt.Sprintf("%s://%s", cu.Scheme, strings.Join(path, "/"))
		}
	}

	return src
}
