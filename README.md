# Go Link Crawler
Crawler for links and titles. It benchmarks requests per second.

Copy `./config/.go-link-crawler.example.yaml` to `./config/.go-link-crawler.yaml`

## Build
`make help`

### Docker
`make run-docker`

if you need use other file than `list.txt` run docker container with
```
docker run -v $PATH_TO_YOUR_FILE:/app/crawl_list.txt -it go-link-crawler ./main ./crawl_list.txt
```

# License
MIT