package services

import (
	"context"
	"crypto/tls"
	"github.com/PuerkitoBio/goquery"
	"go-link-crawler/config"
	"go-link-crawler/log"
	"go-link-crawler/utils"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var crawlerServiceInstance *CrawlerService

var (
	reLink  = regexp.MustCompile(`(?is)<a (?:[^>]+)?href=(?:"|')([^"']+?)(?:"|').+?\/?>`)
	reTitle = regexp.MustCompile(`(?is)<head>.+?<title>(.+?)</title>.+?</head>`)
)

type CrawlerService struct {
	conf       config.CrawlerConfig
	httpClient *http.Client
	ctx        context.Context
	cancel     context.CancelFunc
	mux        sync.RWMutex
}

type CrawlerProcess struct {
	createdAt  time.Time
	uri        *url.URL
	Error      error
	Completed  bool
	sitemap    map[string]int // url -> depth
	data       map[string]crawlerLinkData
	external   map[string]bool
	links      chan crawlerLink
	workers    int32
	linksCount int32
	wg         sync.WaitGroup
	mux        sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
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

// NewCrawlerService returns only first created instance
func NewCrawlerService(conf config.CrawlerConfig) *CrawlerService {
	if crawlerServiceInstance != nil {
		return crawlerServiceInstance
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}

	ctx, cancel := context.WithCancel(context.Background())

	crawlerServiceInstance = &CrawlerService{
		conf:       conf,
		httpClient: client,
		ctx:        ctx,
		cancel:     cancel,
	}

	return crawlerServiceInstance
}

func (s *CrawlerService) Close() {
	s.cancel()
}

func (s *CrawlerService) Start(rawurl string) (*CrawlerProcess, error) {
	uri, err := url.Parse(rawurl)
	if err != nil {
		log.WithTrace("CrawlerService", "Start").Errorf("url.Parse err: %v", err)
		return nil, err
	}

	uri.Host = strings.TrimLeft(uri.Host, "www.")

	p := &CrawlerProcess{
		createdAt:  time.Now(),
		uri:        uri,
		sitemap:    make(map[string]int),
		data:       make(map[string]crawlerLinkData),
		external:   make(map[string]bool),
		links:      make(chan crawlerLink),
		workers:    0,
		linksCount: 0,
		mux:        sync.RWMutex{},
		ctx:        s.ctx,
		cancel:     s.cancel,
	}

	for i := 0; i < s.conf.Workers; i++ {
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
						start := time.Now()
						atomic.AddInt32(&p.workers, 1)
						atomic.AddInt32(&p.linksCount, -1)

						// request body
						res, err := s.httpClient.Get(link.Url)
						if err != nil {
							log.WithTrace("CrawlerService", "Start", "worker").Errorf("s.httpClient.Get link: %s err: %v", link, err)
							atomic.AddInt32(&p.workers, -1)
							continue
						}

						body, err := ioutil.ReadAll(res.Body)
						if err != nil {
							log.WithTrace("CrawlerService", "Start", "worker").Errorf("ioutil.ReadAll link: %s err: %v", link, err)
							atomic.AddInt32(&p.workers, -1)
							continue
						}
						res.Body.Close()

						// get title & links
						var title string
						var links []string

						if s.conf.UseRegexForParsing {
							title = s.ParseReTitle(body)
							links = s.ParseReLinks(body)
						} else {
							gqBody, err := goquery.NewDocumentFromReader(
								strings.NewReader(strings.ToLower(string(body))))
							if err != nil {
								log.WithTrace("CrawlerService", "Start", "worker").Errorf("goquery.NewDocumentFromReader link: %s err: %v", link, err)
								atomic.AddInt32(&p.workers, -1)
								continue
							}

							title = s.ParseGoqueryTitle(gqBody)
							links = s.ParseGoqueryLinks(gqBody)
						}

						// process links
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
								if depth < s.conf.Depth {
									p.mux.Lock()
									if _, ok := p.sitemap[fullUrl]; !ok { // unique inner url
										p.sitemap[fullUrl] = depth
										p.mux.Unlock()

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
								p.mux.Lock()
								p.external[fullUrl] = true
								p.mux.Unlock()
							}
						}

						// there is no links and this is the last worker
						if innerLinksCount == 0 {
							p.mux.Lock()
							if p.workers == 1 && p.linksCount == 0 {
								close(p.links)
							}
							p.mux.Unlock()
						}
						atomic.AddInt32(&p.workers, -1)

						// store data
						p.mux.Lock()
						p.data[link.Url] = crawlerLinkData{
							Title: title,
							Start: start,
							Since: time.Since(start),
						}
						p.mux.Unlock()
					} else {
						return
					}
				}
			}
		}()
	}

	// put the first link
	log.WithTrace("CrawlerService", "Start").Trace("crawl link: ", rawurl)
	p.linksCount = 1
	p.links <- crawlerLink{
		Url:   rawurl,
		Depth: 0,
	}

	return p, nil
}

func (s *CrawlerService) ParseReTitle(body []byte) string {
	matches := reTitle.FindAllSubmatch(body, -1)
	if len(matches) > 0 {
		return string(matches[0][1])
	}
	return ""
}

func (s *CrawlerService) ParseReLinks(body []byte) []string {
	matches := reLink.FindAllSubmatch(body, -1)
	res := make([]string, 0)
	for _, m := range matches {
		res = append(res, string(m[1]))
	}
	return res
}

func (s *CrawlerService) ParseGoqueryTitle(body *goquery.Document) string {
	title := ""
	body.Find("head > title").Each(func(i int, s *goquery.Selection) {
		title = s.Text()
	})
	return title
}

func (s *CrawlerService) ParseGoqueryLinks(body *goquery.Document) []string {
	res := make([]string, 0)
	body.Find("a").Each(func(i int, s *goquery.Selection) {
		if link, ok := s.Attr("href"); ok && link != "" {
			res = append(res, link)
		}
	})
	return res
}
