package services

import (
	"context"
	"crypto/tls"
	"go-link-crawler/config"
	"go-link-crawler/log"
	"net/http"
	"regexp"
	"sync"
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

func (s *CrawlerService) Start(rawUrl string) (*CrawlerProcess, error) {
	p, err := s.newCrawlerProcess(rawUrl)
	if err != nil {
		return p, err
	}

	// worker pools
	for i := 0; i < s.conf.Workers; i++ {
		p.runWorker()
	}

	// put the first link
	log.WithTrace("CrawlerService", "Start").Trace("crawl link: ", rawUrl)
	p.linksCount = 1
	p.links <- crawlerLink{
		Url:   rawUrl,
		Depth: 0,
	}

	return p, nil
}
