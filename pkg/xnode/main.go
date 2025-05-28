package xnode

import (
	"bytes"
	"strings"

	"github.com/andybalholm/cascadia"
	"github.com/yosssi/gohtml"
	"golang.org/x/net/html"
)

type Node struct {
	*html.Node
}

func NewNode(b []byte) (*Node, error) {
	doc, err := html.Parse(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	return &Node{doc}, nil
}

func (n *Node) NextSibling() *Node {
	return &Node{n.Node.NextSibling}
}
func (n *Node) NextSiblingWithAttr(query string) *Node {
	var k, v string
	if strings.Contains(query, "=") {
		k = strings.Split(query, "=")[0]
		v = strings.Split(query, "=")[1]
	} else {
		k = query
		v = ""
	}
	for c := n.Node.NextSibling; c != nil; c = c.NextSibling {
		c_ := &Node{c}
		if len(v) == 0 {
			_, ok := c_.GetAttribute(k)
			if ok {
				return c_
			}
		} else {
			if c_.check(k, v) {
				return c_
			}
		}
	}

	return &Node{n.Node.NextSibling}
}
func (n *Node) FirstChildWithAttr(query string) *Node {
	var k, v string
	if strings.Contains(query, "=") {
		k = strings.Split(query, "=")[0]
		v = strings.Split(query, "=")[1]
	} else {
		k = query
		v = ""
	}
	for c := n.Node.FirstChild; c != nil; c = c.NextSibling {
		c_ := &Node{c}
		if len(v) == 0 {
			_, ok := c_.GetAttribute(k)
			if ok {
				return c_
			}
		} else {
			if c_.check(k, v) {
				return c_
			}
		}
	}

	return &Node{n.Node.NextSibling}
}
func (n *Node) FirstChild() *Node {
	return &Node{n.Node.FirstChild}
}

func (n *Node) Children() NodeList {
	var children NodeList
	for c := n.Node.FirstChild; c != nil; c = c.NextSibling {
		c_ := &Node{c}
		children = append(children, c_)
	}
	return children
}

func (n *Node) GetAttribute(key string) (string, bool) {
	for _, attr := range n.Node.Attr {
		if attr.Key == key {
			return attr.Val, true
		}
	}
	return "", false
}

func (n *Node) Name() string {
	name, _ := n.GetAttribute("name")
	return name
}
func (n *Node) Class() string {
	class, _ := n.GetAttribute("class")
	return class
}
func (n *Node) Id() string {
	id, _ := n.GetAttribute("id")
	return id
}
func (n *Node) Src() string {
	src, _ := n.GetAttribute("src")
	return src
}
func (n *Node) Href() string {
	href, _ := n.GetAttribute("href")
	return href
}

func (n *Node) check(k, v string) bool {
	if n.Type == html.ElementNode {
		s, ok := n.GetAttribute(k)
		if ok && s == v {
			return true
		}
	}
	return false
}

func (n *Node) traverse(k, v string) *Node {
	if n.check(k, v) {
		return n
	}

	for c := n.Node.FirstChild; c != nil; c = c.NextSibling {
		c_ := &Node{c}
		result := c_.traverse(k, v)
		if result != nil {
			return result
		}
	}

	return nil
}

func (n *Node) GetElementById(id string) *Node {
	return n.traverse("id", id)
}
func (n *Node) GetElementByName(name string) *Node {
	return n.traverse("name", name)
}

func (n *Node) traverseAndCollect(k, v string, collector *NodeList) {
	if n.check(k, v) {
		*collector = append(*collector, n)
	}

	for c := n.Node.FirstChild; c != nil; c = c.NextSibling {
		c_ := &Node{c}
		c_.traverseAndCollect(k, v, collector)
	}
}

func (n *Node) GetElementsById(id string) NodeList {
	var collector NodeList
	n.traverseAndCollect("id", id, &collector)
	return collector
}

func (n *Node) GetElementsByName(name string) NodeList {
	var collector NodeList
	n.traverseAndCollect("name", name, &collector)
	return collector
}

func (n *Node) QuerySelector(selectorStr string) *Node {
	selector, err := cascadia.Compile(selectorStr)
	if err != nil {
		return nil
	}
	matched := selector.MatchFirst(n.Node)
	if matched == nil {
		return nil // No match found
	}
	return &Node{matched}
}

func (n *Node) QuerySelectorAll(selectorStr string) NodeList {
	selector, err := cascadia.Compile(selectorStr)
	if err != nil {
		return nil
	}
	matches := selector.MatchAll(n.Node)
	nodes := make([]*Node, len(matches))
	for i, match := range matches {
		nodes[i] = &Node{match}
	}
	return nodes
}

type NodeList []*Node

func (n *Node) Find(selectorStr string) NodeList {
	selector, err := cascadia.Compile(selectorStr)
	if err != nil {
		return nil
	}
	matches := selector.MatchAll(n.Node)
	nodes := make([]*Node, len(matches))
	for i, match := range matches {
		nodes[i] = &Node{match}
	}
	return nodes
}

func (list NodeList) Each(callback func(i int, n *Node)) {
	for i, node := range list {
		callback(i, node)
	}
}

func (n *Node) Attrs() map[string]string {
	attrs := make(map[string]string)
	for _, attr := range n.Node.Attr {
		attrs[attr.Key] = attr.Val
	}
	return attrs
}

func (n *Node) Attr(key string) string {
	val, _ := n.GetAttribute(key)
	return val
}

func getData(n *html.Node) string {
	if n == nil {
		return ""
	}
	if n.FirstChild == nil {
		return ""
	}
	if n.FirstChild.Type != html.TextNode {
		return getData(n.FirstChild)
	}
	return strings.TrimSpace(n.FirstChild.Data)
}

func (n *Node) Text() string {
	return getData(n.Node)
}

func (n *Node) Next() *Node {
	nn := &Node{n.Node.NextSibling}
	if len(strings.TrimSpace(nn.String())) == 0 {
		return nn.Next()
	}
	return nn
}

func (n *Node) PrettyPrintHTML() string {
	var buffer bytes.Buffer
	err := html.Render(&buffer, n.Node)
	if err != nil {
		return "" // handle error as you see fit
	}
	return gohtml.Format(buffer.String())
}

func (n *Node) String() string {
	var buffer bytes.Buffer
	err := html.Render(&buffer, n.Node)
	if err != nil {
		return "" // handle error as you see fit
	}
	return buffer.String()
}
