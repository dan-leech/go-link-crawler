package main

import (
	"go-link-crawler/config"
	"go-link-crawler/log"
	"go-link-crawler/services"
	"io/ioutil"
	"os"
	"strings"
)

func main() {
	// set log level
	log.SetLevel(log.TraceLevel)

	conf := config.Init()
	crawler := services.NewCrawlerService(conf.CrawlerConfig)

	if len(os.Args) < 2 {
		log.Fatalf("use filepath as first argument")
	}

	arg := os.Args[1]
	f, err := os.Open(arg)
	if err != nil {
		log.Fatalf("cannot open %s err: %v", arg, err)
	}

	data, err := ioutil.ReadAll(f)
	if err != nil {
		log.Fatalf("cannot read %s err: %v", arg, err)
	}

	links := strings.Split(string(data), "\n")
	crawlerProcesses := make([]*services.CrawlerProcess, 0)
	for _, link := range links {
		p, err := crawler.Start(link)
		if err != nil {
			log.Errorf("crawler.Start err: %v", err)
		}

		crawlerProcesses = append(crawlerProcesses, p)
	}

	// get results
	var req float32
	count := 0
	req = 0
	for _, p := range crawlerProcesses {
		res := p.GetResult()
		//j, err := json.Marshal(res)
		//if err != nil {
		//	log.Errorf("json.Marshal(res) err: %v", err)
		//	continue
		//}
		log.Infof("Domain: %s, Links count: %d, External links count: %d, req/sec: %.2f", res.Domain, res.InnerLinksCount, res.ExternalLinksCount, res.RequestsPerSec)
		count++
		req = req + res.RequestsPerSec
	}

	if count > 0 {
		req = req / float32(count)
	}

	log.Infof("requests/sec: %.2f", req)

	crawler.Close()
}
