package pdf

import (
	"errors"
	"strings"

	xhtml "golang.org/x/net/html"
)

const contentSecurityPolicy = "default-src 'none'; img-src data:; style-src 'unsafe-inline'; font-src data:; media-src data:; object-src 'none'; frame-src 'none'; base-uri 'none'; form-action 'none'"

var blockedElements = map[string]struct{}{
	"base":     {},
	"embed":    {},
	"frame":    {},
	"frameset": {},
	"iframe":   {},
	"link":     {},
	"object":   {},
	"script":   {},
}

var urlAttributes = map[string]struct{}{
	"action":     {},
	"background": {},
	"cite":       {},
	"formaction": {},
	"href":       {},
	"poster":     {},
	"src":        {},
	"xlink:href": {},
}

func secureHTML(source string) (string, error) {
	document, err := xhtml.Parse(strings.NewReader(source))
	if err != nil {
		return "", err
	}

	sanitizeNode(document)

	head := findElement(document, "head")
	if head == nil {
		return "", errors.New("parsed document has no head element")
	}

	csp := &xhtml.Node{
		Type: xhtml.ElementNode,
		Data: "meta",
		Attr: []xhtml.Attribute{
			{Key: "http-equiv", Val: "Content-Security-Policy"},
			{Key: "content", Val: contentSecurityPolicy},
		},
	}
	head.InsertBefore(csp, head.FirstChild)

	var output strings.Builder
	if err := xhtml.Render(&output, document); err != nil {
		return "", err
	}
	return output.String(), nil
}

func sanitizeNode(node *xhtml.Node) {
	for child := node.FirstChild; child != nil; {
		next := child.NextSibling
		if shouldRemoveElement(child) {
			node.RemoveChild(child)
			child = next
			continue
		}

		if child.Type == xhtml.ElementNode {
			child.Attr = sanitizeAttributes(child.Attr)
		}
		sanitizeNode(child)
		child = next
	}
}

func shouldRemoveElement(node *xhtml.Node) bool {
	if node.Type != xhtml.ElementNode {
		return false
	}

	name := strings.ToLower(node.Data)
	if _, blocked := blockedElements[name]; blocked {
		return true
	}
	if name != "meta" {
		return false
	}

	for _, attribute := range node.Attr {
		if strings.EqualFold(attribute.Key, "http-equiv") && strings.EqualFold(strings.TrimSpace(attribute.Val), "refresh") {
			return true
		}
	}
	return false
}

func sanitizeAttributes(attributes []xhtml.Attribute) []xhtml.Attribute {
	safe := attributes[:0]
	for _, attribute := range attributes {
		key := strings.ToLower(attribute.Key)
		qualifiedKey := key
		if attribute.Namespace != "" {
			qualifiedKey = strings.ToLower(attribute.Namespace + ":" + attribute.Key)
		}
		if strings.HasPrefix(key, "on") || key == "srcset" {
			continue
		}
		_, localURL := urlAttributes[key]
		_, qualifiedURL := urlAttributes[qualifiedKey]
		if (localURL || qualifiedURL) && !isEmbeddedURL(attribute.Val) {
			continue
		}
		safe = append(safe, attribute)
	}
	return safe
}

func isEmbeddedURL(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || strings.HasPrefix(value, "#") {
		return true
	}
	return strings.HasPrefix(strings.ToLower(value), "data:")
}

func findElement(node *xhtml.Node, name string) *xhtml.Node {
	if node.Type == xhtml.ElementNode && strings.EqualFold(node.Data, name) {
		return node
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if match := findElement(child, name); match != nil {
			return match
		}
	}
	return nil
}
