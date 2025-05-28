package lib

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/kiyor/k2fs/pkg/xnode_client" // Corrected import path
	"golang.org/x/net/proxy"
)

type SearchClient struct {
	*http.Client
}

func NewSearchClient() *SearchClient {
	dialer, _ := proxy.SOCKS5("tcp", "192.168.10.10:1080", nil, proxy.Direct)
	transport := &http.Transport{
		Dial:            dialer.Dial,
		IdleConnTimeout: 30 * time.Second,
	}
	return &SearchClient{
		Client: &http.Client{
			Transport: transport,
			Timeout:   10 * time.Second,
		},
	}
}

type SearchResult struct {
	Name  string
	Id    string
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

// name=FC2-PPV-123456
func (s *SearchClient) Search(name string) (*SearchResult, error) {
	log.Println("------ perform search", name)
	for strings.Contains(name, "--") {
		name = strings.ReplaceAll(name, "--", "-")
	}
	key := "SEARCH:" + name
	var res SearchResult
	if ok := Redis.GetValue(key, &res); ok {
		log.Println("SEARCH", name, "HIT")
		return &res, nil
	}
	log.Println("SEARCH", name, "MISS")

	reid := regexp.MustCompile(`-(\d+)$`)
	if reid.MatchString(name) {
		id := reid.FindStringSubmatch(name)[1]
		if len(id) > 0 {
			res.Id = id
		}
	}

	k := name

	for _, name := range []string{name, strings.ToUpper(name), strings.ToLower(name)} {
		for _, config := range SearchConfigList {
			for i := 0; i < 2; i++ {
				log.Println("REQUEST", config.Prefix+name)
				req, err := http.NewRequest("GET", config.Prefix+name, nil)
				if err != nil {
					return nil, err
				}
				req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 11_1_0) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/87.0.4280.141 Safari/537.36")
				resp, err := s.Do(req)
				if err != nil {
					return nil, err
				}
				defer resp.Body.Close()
				if resp.StatusCode == 429 {
					log.Println(config.Prefix+name, "429, sleep 2s")
					time.Sleep(2 * time.Second)
					continue
				}
				if resp.StatusCode != 200 {
					return nil, fmt.Errorf("status code: %d", resp.StatusCode)
				}
				b, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, err
				}
				// 			log.Println(string(b))
				node, _ := xnode_client.NewNode(b) // Corrected package name
				var title string
				node.Find(`a`).Each(func(i int, n *xnode_client.Node) { // Corrected type
					if n.Attr("title") != "" {
						if strings.HasPrefix(n.Attr("href"), "/view/") {
							title = n.Attr("title")
						}
					}
				})
				if len(title) > 0 {
					log.Printf("fc2-%s: %s\n", res.Id, title)
					res.Name = k
					part := strings.Split(title, res.Id)
					if len(part) > 1 {
						res.Title = strings.TrimSpace(part[1])
					} else {
						res.Title = strings.TrimSpace(title)
					}
					// have value, cache 30 days
					Redis.SetValueWithTTL(key, res, 2592000)
					return &res, nil
				}
				break
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
