package services

import "time"

type crawlerResult struct {
	Domain             string
	Sitemap            map[string]string `json:"sitemap"`
	InnerLinksCount    int               `json:"inner_links_count"`
	ExternalLinks      []string          `json:"external_links"`
	ExternalLinksCount int               `json:"external_links_count"`
	RequestsPerSec     float32           `json:"requests_per_sec"`
}

func (p *CrawlerProcess) RequestsPerSec() float32 {
	start := time.Now()
	end := time.Now().AddDate(-1, 0, 0)
	requests := 0
	for _, d := range p.data {
		if d.Start.Before(start) {
			start = d.Start
		}
		s := d.Start.Add(d.Since)
		if s.After(end) {
			end = s
		}
		requests++
	}

	d := end.Sub(start)
	return float32(requests) * 1000000000 / float32(d.Nanoseconds())
}

func (p *CrawlerProcess) GetResult() crawlerResult {
	p.wg.Wait()

	res := crawlerResult{
		Domain:         p.uri.Host,
		Sitemap:        map[string]string{},
		ExternalLinks:  []string{},
		RequestsPerSec: 0,
	}

	for l, d := range p.data {
		res.Sitemap[l] = d.Title
		res.InnerLinksCount++
	}

	for l, _ := range p.external {
		res.ExternalLinks = append(res.ExternalLinks, l)
		res.ExternalLinksCount++
	}

	res.RequestsPerSec = p.RequestsPerSec()

	return res
}
