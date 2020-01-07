package services

import (
	"context"
	"github.com/PuerkitoBio/goquery"
	"go-link-crawler/log"
	"go-link-crawler/utils"
	"io/ioutil"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type CrawlerProcess struct {
	crawlerService *CrawlerService
	createdAt      time.Time
	uri            *url.URL
	Error          error
	Completed      bool
	sitemap        map[string]int // url -> depth
	data           map[string]crawlerLinkData
	external       map[string]bool
	links          chan crawlerLink
	workers        int32
	linksCount     int32
	wg             sync.WaitGroup
	mux            sync.RWMutex
	ctx            context.Context
	cancel         context.CancelFunc
}

type crawlerLink struct {
	Url   string
	Depth int
}

type crawlerLinkData struct {
	Title string
	Start time.Time
	Since time.Duration
}

func (s *CrawlerService) newCrawlerProcess(rawUrl string) (*CrawlerProcess, error) {
	uri, err := url.Parse(rawUrl)
	if err != nil {
		log.WithTrace("CrawlerService", "newCrawlerProcess").Errorf("url.Parse err: %v", err)
		return nil, err
	}

	uri.Host = strings.TrimLeft(uri.Host, "www.")

	return &CrawlerProcess{
		crawlerService: s,
		createdAt:      time.Now(),
		uri:            uri,
		sitemap:        make(map[string]int),
		data:           make(map[string]crawlerLinkData),
		external:       make(map[string]bool),
		links:          make(chan crawlerLink),
		workers:        0,
		linksCount:     0,
		mux:            sync.RWMutex{},
		ctx:            s.ctx,
		cancel:         s.cancel,
	}, nil
}

func (p *CrawlerProcess) runWorker() {
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		for {
			select {
			case <-p.ctx.Done():
				log.WithTrace("CrawlerService", "Start", "worker").Debug("worker canceled by context")
				return
			case link, more := <-p.links:
				if more {
					if err := p.processLink(link); err != nil {
						continue
					}
				} else {
					return
				}
			}
		}
	}()
}

func (p *CrawlerProcess) processLink(link crawlerLink) error {
	log.WithTrace("CrawlerService", "CrawlerProcess", "processLink").Tracef("start processing link: %s", link.Url)

	start := time.Now()
	atomic.AddInt32(&p.workers, 1)
	defer atomic.AddInt32(&p.workers, -1)
	atomic.AddInt32(&p.linksCount, -1)

	// request body
	body, err := p.requestBody(link)
	if err != nil {
		return err
	}

	// get title & links
	title, links, err := p.parseData(body)
	if err != nil {
		log.WithTrace("CrawlerService", "CrawlerProcess", "processLink").Errorf("p.parseData(body) link: %s err: %v", link.Url, err)
		return err
	}

	log.WithTrace("CrawlerService", "CrawlerProcess", "processLink").Tracef("title: %s link: %s", title, link.Url)

	// process new links
	p.processNewLinks(link, links)

	// store data
	p.mux.Lock()
	p.data[link.Url] = crawlerLinkData{
		Title: title,
		Start: start,
		Since: time.Since(start),
	}
	p.mux.Unlock()

	return nil
}

func (p *CrawlerProcess) requestBody(link crawlerLink) ([]byte, error) {
	res, err := p.crawlerService.httpClient.Get(link.Url)
	if err != nil {
		log.WithTrace("CrawlerService", "CrawlerProcess", "requestBody").Errorf("p.crawlerService.httpClient.Get link: %s err: %v", link.Url, err)
		return nil, err
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.WithTrace("CrawlerService", "CrawlerProcess", "requestBody").Errorf("ioutil.ReadAll link: %s err: %v", link.Url, err)
		return nil, err
	}
	res.Body.Close()

	return body, nil
}

func (p *CrawlerProcess) processNewLinks(link crawlerLink, links []string) {
	innerLinksCount := 0
	for _, l := range links {
		// check that it is correct url scheme
		scheme := utils.GetUrlScheme(l)
		if scheme != "http" && scheme != "https" && scheme != "" {
			continue
		}

		fullUrl := utils.RelativeUrlToFull(l, link.Url, p.uri)

		if utils.IsInnerUrl(fullUrl, p.uri) {
			depth := link.Depth + 1
			if depth < p.crawlerService.conf.Depth {
				p.mux.Lock()
				if _, ok := p.sitemap[fullUrl]; !ok { // unique inner url
					p.sitemap[fullUrl] = depth
					p.mux.Unlock()

					log.WithTrace("CrawlerService", "CrawlerProcess", "processLink").Tracef("new inner link found: %s on link request: %s", fullUrl, link.Url)

					innerLinksCount++
					atomic.AddInt32(&p.linksCount, 1)
					go func() { // prevent deadlock waiting
						p.links <- crawlerLink{
							Url:   fullUrl,
							Depth: link.Depth + 1,
						}
					}()
				} else {
					p.mux.Unlock()
				}
			}
		} else {
			log.WithTrace("CrawlerService", "CrawlerProcess", "processLink").Tracef("new external link found: %s on link request: %s", fullUrl, link.Url)
			p.mux.Lock()
			p.external[fullUrl] = true
			p.mux.Unlock()
		}
	}

	// there is no links and this is the last worker
	if innerLinksCount == 0 {
		p.mux.Lock()
		if p.workers == 1 && p.linksCount == 0 {
			log.WithTrace("CrawlerService", "CrawlerProcess", "processLink").Tracef("There is no more links on %s => finish", p.uri.String())
			close(p.links)
		}
		p.mux.Unlock()
	}
}

func (p *CrawlerProcess) parseData(body []byte) (string, []string, error) {
	var title string
	var links []string

	if p.crawlerService.conf.UseRegexForParsing {
		title = p.parseReTitle(body)
		links = p.parseReLinks(body)
	} else {
		gqBody, err := goquery.NewDocumentFromReader(
			strings.NewReader(strings.ToLower(string(body))))
		if err != nil {
			return "", nil, err
		}

		title = p.parseGoqueryTitle(gqBody)
		links = p.parseGoqueryLinks(gqBody)
	}

	return title, links, nil
}

func (p *CrawlerProcess) parseReTitle(body []byte) string {
	matches := reTitle.FindAllSubmatch(body, -1)
	if len(matches) > 0 {
		return string(matches[0][1])
	}
	return ""
}

func (p *CrawlerProcess) parseReLinks(body []byte) []string {
	matches := reLink.FindAllSubmatch(body, -1)
	res := make([]string, 0)
	for _, m := range matches {
		res = append(res, string(m[1]))
	}
	return res
}

func (p *CrawlerProcess) parseGoqueryTitle(body *goquery.Document) string {
	title := ""
	body.Find("head > title").Each(func(i int, s *goquery.Selection) {
		title = s.Text()
	})
	return title
}

func (p *CrawlerProcess) parseGoqueryLinks(body *goquery.Document) []string {
	res := make([]string, 0)
	body.Find("a").Each(func(i int, s *goquery.Selection) {
		if link, ok := s.Attr("href"); ok && link != "" {
			res = append(res, link)
		}
	})
	return res
}
