package statuspage

import (
	"fmt"
	"net/http"
	"reflect"
	"strconv"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// Status implements net/http.Handler, and provides a status page for the value returned by cb
type Status[T any] struct {
	title string
	cb    func() T
}

func (s *Status[T]) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	v := s.cb()
	rootN, genErr := s.genTopLevelHTML(reflect.ValueOf(v))
	if genErr != nil {
		http.Error(w, fmt.Sprintf("failed to generate HTML for struct of type %T: %s", v, genErr), 500)
		return
	}
	if renderErr := html.Render(w, rootN); renderErr != nil {
		http.Error(w, fmt.Sprintf("failed to render response for struct of type %T: %s", v, genErr), 500)
	}
}

func (s *Status[T]) genTopLevelHTML(v reflect.Value) (*html.Node, error) {
	root := html.Node{Type: html.DocumentNode}
	root.AppendChild(&html.Node{
		Type:     html.DoctypeNode,
		DataAtom: atom.Html,
		Data:     atom.Html.String(),
	})
	htmlElem := createElemAtom(atom.Html)
	root.AppendChild(htmlElem)

	head := createElemAtom(atom.Head)

	htmlElem.AppendChild(head)
	title := createElemAtom(atom.Title)
	title.AppendChild(textNode(s.title))
	head.AppendChild(title)
	header := createElemAtom(atom.H1)
	header.AppendChild(textNode(s.title))
	head.AppendChild(header)

	body := createElemAtom(atom.Body)
	htmlElem.AppendChild(body)

	// TODO: add CSS references, etc. to the HEAD element
	bodyNodes, bodyGenErr := s.genValSections(v)
	if bodyGenErr != nil {
		return nil, bodyGenErr
	}
	for _, bn := range bodyNodes {
		// add a horizontal rule to separate sections
		body.AppendChild(createElemAtom(atom.Hr))
		body.AppendChild(bn)
	}

	return &root, nil
}

func (s *Status[T]) genValSections(v reflect.Value) ([]*html.Node, error) {
	k := v.Kind()
	switch k {
	case reflect.Struct:
	case reflect.Map:
	case reflect.Array, reflect.Slice:
	case reflect.Pointer:
	case reflect.UnsafePointer:
	case reflect.Chan:
	case reflect.Interface:
		// primitive types that don't require any recursion/traversal
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128, reflect.String, reflect.Func:
		ns, nErr := s.genValSection(v)
		if nErr != nil {
			return nil, fmt.Errorf("failed to generate primitive fields: %w", nErr)
		}
		return ns, nil
	default:
		panic(fmt.Sprintf("unhandled kind %s (type %T)", k, v.Type()))
	}
	panic("unimplemented")
}

func (s *Status[T]) genValSection(v reflect.Value) ([]*html.Node, error) {
	k := v.Kind()
	switch k {
	case reflect.Struct:
		ns, tblErr := s.genStructTable(v)
		if tblErr != nil {
			return nil, tblErr
		}

		return ns, nil
	case reflect.Map:
	case reflect.Array, reflect.Slice:
	case reflect.Pointer:
		if v.IsNil() {
			return []*html.Node{textNode(v.Type().String() + "(nil)")}, nil
		}
		// Delegate after following the bouncing ball
		return s.genValSection(v.Elem())
	case reflect.Bool:
		// TODO: wrap this text node in an element node so we can key some CSS styling
		return []*html.Node{textNode(strconv.FormatBool(v.Bool()))}, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		// TODO: wrap this text node in an element node so we can key some CSS styling
		return []*html.Node{textNode(strconv.FormatInt(v.Int(), 10) + " (" + strconv.FormatInt(v.Int(), 16) + ")")}, nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		// TODO: wrap this text node in an element node so we can key some CSS styling
		return []*html.Node{textNode(strconv.FormatUint(v.Uint(), 10) + " (" + strconv.FormatUint(v.Uint(), 16) + ")")}, nil
	case reflect.UnsafePointer:
		vp := v.UnsafePointer()
		return []*html.Node{textNode(strconv.FormatUint(uint64(uintptr(vp)), 10) + " (" + strconv.FormatUint(uint64(uintptr(vp)), 16) + ")")}, nil
	case reflect.Float32, reflect.Float64:
		bs := 32
		if k == reflect.Float64 {
			bs = 64
		}
		// TODO: wrap this text node in an element node so we can key some CSS styling
		return []*html.Node{textNode(strconv.FormatFloat(v.Float(), 'g', -1, bs))}, nil
	case reflect.Complex64, reflect.Complex128:
		bs := 64
		if k == reflect.Complex128 {
			bs = 128
		}
		return []*html.Node{textNode(strconv.FormatComplex(v.Complex(), 'g', -1, bs))}, nil
	case reflect.String:
		return []*html.Node{{
			Type: html.TextNode,
			Data: v.String(),
		}}, nil
	case reflect.Chan:
	case reflect.Func:
	case reflect.Interface:

	default:
		panic(fmt.Sprintf("unhandled kind %s (type %T)", k, v.Type()))
	}
	panic("unimplemented")
}
