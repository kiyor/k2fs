package lib

import (
	"fmt"
	"io"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/kiyor/k2fs/pkg/xnode"
)

type SearchClient struct {
	*retryablehttp.Client
}

func NewSearchClient() *SearchClient {
	c := retryablehttp.NewClient()
	c.HTTPClient.Timeout = 2 * time.Second
	c.RetryMax = 1
	return &SearchClient{
		Client: c,
	}
}

type SearchResult struct {
	Name  string
	Title string
}

type SearchConfig struct {
	Prefix string
	Regex  *regexp.Regexp
}

var SearchConfigList = []*SearchConfig{
	{
		Prefix: "https://sukebei.nyaa.si/user/offkab?f=0&c=0_0&q=",
		Regex:  regexp.MustCompile(`<a href="/view/\d+" title="(.*)">[\+\s]+[^\s]+\s(.*)</a>`),
	},
	{
		Prefix: "https://sukebei.nyaa.si/?f=0&c=0_0&q=",
		Regex:  regexp.MustCompile(`<a href="/view/\d+" title="(.*)">(.*)</a>`),
	},
}

var urlPrefix = "https://sukebei.nyaa.si/user/offkab?f=0&c=0_0&q="

func (s *SearchClient) Search(name string) (*SearchResult, error) {
	for strings.Contains(name, "--") {
		name = strings.ReplaceAll(name, "--", "-")
	}

	key := "SEARCH:" + name
	var res SearchResult
	if ok := Redis.GetValue(key, &res); ok {
		// log.Println("SEARCH", name, "HIT")
		return &res, nil
	}
	log.Println("SEARCH", name, "MISS")

	k := name

	for _, name := range []string{name, strings.ToUpper(name), strings.ToLower(name)} {
		for _, config := range SearchConfigList {
			log.Println("REQUEST", config.Prefix+name)
			req, err := retryablehttp.NewRequest("GET", config.Prefix+name, nil)
			if err != nil {
				return nil, err
			}
			req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 11_1_0) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/87.0.4280.141 Safari/537.36")
			resp, err := s.Do(req)
			if err != nil {
				return nil, err
			}
			defer resp.Body.Close()
			b, err := io.ReadAll(resp.Body)
			if err != nil {
				return nil, err
			}
			// 			log.Println(string(b))
			node, _ := xnode.NewNode(b)
			var title string
			node.Find(`a`).Each(func(i int, n *xnode.Node) {
				if n.Attr("title") != "" {
					if strings.HasPrefix(n.Attr("href"), "/view/") {
						title = n.Attr("title")
					}
				}
			})
			if len(title) > 0 {
				log.Println(title)
				res = SearchResult{
					Name:  k,
					Title: title,
				}
				// have value, cache 30 days
				Redis.SetValueWithTTL(key, res, 2592000)
				return &res, nil
			}
		}
	}

	res = SearchResult{
		Name:  name,
		Title: "",
	}
	// not found, cache 10 days
	Redis.SetValueWithTTL(key, res, 864000)
	return nil, fmt.Errorf("not found")
}
