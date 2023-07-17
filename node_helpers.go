package statuspage

import (
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

func createElemAtom(d atom.Atom) *html.Node {
	return &html.Node{Type: html.ElementNode, DataAtom: d, Data: d.String()}
}

func textNode(d string) *html.Node {
	return &html.Node{Type: html.TextNode, Data: d}
}
