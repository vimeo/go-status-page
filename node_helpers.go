package statuspage

import (
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

func createElemAtom(d atom.Atom) *html.Node {
	n := &html.Node{Type: html.ElementNode, DataAtom: d, Data: d.String()}
	if n.DataAtom == atom.Table {
		n.Attr = append(n.Attr, html.Attribute{
			Key: "style",
			Val: "border: 1px solid; min-width: 100px",
		})
	}
	return n
}

func textNode(d string) *html.Node {
	return &html.Node{Type: html.TextNode, Data: d}
}
