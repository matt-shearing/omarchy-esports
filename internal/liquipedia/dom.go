package liquipedia

import (
	"strings"

	"golang.org/x/net/html"
)

// Small DOM helpers over x/net/html. Liquipedia's ticker markup is generated
// by wiki templates and is structurally regular, but it is not valid XML and
// class attributes carry several space-separated tokens, so we match on token
// membership rather than string equality.

func attr(n *html.Node, key string) string {
	if n == nil {
		return ""
	}
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

// hasClass reports whether n carries the given class token.
func hasClass(n *html.Node, class string) bool {
	if n == nil || n.Type != html.ElementNode {
		return false
	}
	for _, f := range strings.Fields(attr(n, "class")) {
		if f == class {
			return true
		}
	}
	return false
}

// findAll walks the subtree collecting every node satisfying pred.
func findAll(n *html.Node, pred func(*html.Node) bool) []*html.Node {
	var out []*html.Node
	var walk func(*html.Node)
	walk = func(cur *html.Node) {
		if cur == nil {
			return
		}
		if pred(cur) {
			out = append(out, cur)
		}
		for c := cur.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return out
}

// find returns the first node satisfying pred, or nil.
func find(n *html.Node, pred func(*html.Node) bool) *html.Node {
	var out *html.Node
	var walk func(*html.Node) bool
	walk = func(cur *html.Node) bool {
		if cur == nil {
			return false
		}
		if pred(cur) {
			out = cur
			return true
		}
		for c := cur.FirstChild; c != nil; c = c.NextSibling {
			if walk(c) {
				return true
			}
		}
		return false
	}
	walk(n)
	return out
}

// byClass builds a predicate matching elements carrying a class token.
func byClass(class string) func(*html.Node) bool {
	return func(n *html.Node) bool { return hasClass(n, class) }
}

// findByClass returns the first descendant with the given class.
func findByClass(n *html.Node, class string) *html.Node {
	return find(n, byClass(class))
}

// findAllByClass returns every descendant with the given class.
func findAllByClass(n *html.Node, class string) []*html.Node {
	return findAll(n, byClass(class))
}

// text extracts the concatenated, whitespace-collapsed text of a subtree.
func text(n *html.Node) string {
	if n == nil {
		return ""
	}
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(cur *html.Node) {
		if cur.Type == html.TextNode {
			b.WriteString(cur.Data)
		}
		for c := cur.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.Join(strings.Fields(b.String()), " ")
}

// firstTag returns the first descendant element with the given tag name.
func firstTag(n *html.Node, tag string) *html.Node {
	return find(n, func(c *html.Node) bool {
		return c.Type == html.ElementNode && c.Data == tag
	})
}
