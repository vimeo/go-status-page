package statuspage

import (
	"fmt"
	"reflect"
	"strconv"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// column header names for maps
const mapKeyHeader = "key"
const mapValueHeader = "value"

// max len of elements shown from a slice if the map value is slice
const maxSliceLen = 5

// isSet checks if t is a map where the value type is struct{}
func isSet(t reflect.Type) bool {
	return t.Kind() == reflect.Map && t.Elem().Kind() == reflect.Struct && t.Elem().NumField() == 0
}

// only works for v where the type is simple (doesn't need a table)
func (s *Status[T]) simpleTableCell(v reflect.Value) (*html.Node, error) {
	cell := createElemAtom(atom.Td)
	ns, genErr := s.genValSection(v)
	if genErr != nil {
		return nil, genErr
	}
	for _, n := range ns {
		cell.AppendChild(n)
	}
	return cell, nil
}

func (s *Status[T]) genMapOrSeq2Table(v reflect.Value) ([]*html.Node, error) {
	if v.Kind() != reflect.Map && (v.Kind() != reflect.Func || v.Type().CanSeq2()) {
		panic(fmt.Errorf("non-map/seq2 kind: %s type %s", v.Kind(), v.Type()))
	}

	baseTable := createElemAtom(atom.Table)
	headerRow := createElemAtom(atom.Tr)
	baseTable.AppendChild(headerRow)

	keyHeader := createElemAtom(atom.Th)
	headerRow.AppendChild(keyHeader)
	keyHeader.AppendChild(textNode(mapKeyHeader))

	valHeader := createElemAtom(atom.Th)
	valHeader.Attr = append(valHeader.Attr, html.Attribute{Key: "colspan", Val: "100%"})
	headerRow.AppendChild(valHeader)
	valHeader.AppendChild(textNode(mapValueHeader))

	var valType reflect.Type
	switch v.Kind() {
	case reflect.Map:
		valType = v.Type().Elem()
	case reflect.Func:
		// this has to be an iter.Seq2
		valType = v.Type().In(0).In(1)
	default:
		panic(fmt.Errorf("non-map/func kind: %s type %s", v.Kind(), v.Type()))
	}

	valSet := isSet(valType)
	if valSet {
		// if valType is a map[someType]struct{}, values are a slice of someType (key type)
		valType = reflect.SliceOf(valType.Key())
	}

	// header row for the map key if applicable
	var hRowKey *html.Node
	for ikey, ival := range v.Seq2() {
		if valSet {
			// ival is actually a slice of valType where the values are ival's map keys, make it so!
			// ex. map[string]struct{}{"dog":struct{}, "cat":struct{}} => ival should be []string{"dog", "cat"}
			keys := ival.MapKeys()
			ival = reflect.MakeSlice(valType, ival.Len(), ival.Len())
			for i, key := range keys {
				ival.Index(i).Set(key)
			}
		}

		row := createElemAtom(atom.Tr)

		// add cells in this row from each key and value
		// indexed by their "level"
		headerRows := []*html.Node{}

		// cells for keys
		// TODO: pull this out into helper
		if !needsTable(ikey.Type()) {
			cell, cellErr := s.simpleTableCell(ikey)
			if cellErr != nil {
				return nil, cellErr
			}
			row.AppendChild(cell)
		} else if ikey.Kind() == reflect.Struct {
			fields := reflect.VisibleFields(ikey.Type())
			hRowKey = createElemAtom(atom.Tr)
			keyHeader.Attr = append(keyHeader.Attr, html.Attribute{Key: atom.Colspan.String(), Val: strconv.Itoa(len(fields))})

			for _, field := range fields {
				if !field.IsExported() {
					// skip the unexported fields for now
					continue
				}
				// TODO: pull out into a helper
				if v, ok := field.Tag.Lookup("statuspage"); ok && v == "-" {
					// The caller asked us to skip this
					continue
				}
				if needsTable(field.Type) {
					// TODO: add in recursion with depth for header row levels, but for now, just stick in a table within this table
					ns, genErr := s.genValSection(ikey.FieldByIndex(field.Index))
					if genErr != nil {
						return nil, genErr
					}
					for _, n := range ns {
						cell := createElemAtom(atom.Td)
						row.AppendChild(cell)
						cell.AppendChild(n)
					}
				}
				th := createElemAtom(atom.Th)
				hRowKey.AppendChild(th)
				th.AppendChild(textNode(field.Name))

				// add the field values from this struct to the values row
				if needsTable(field.Type) {
					continue
				}
				fieldValCell, cellErr := s.simpleTableCell(ikey.FieldByName(field.Name))
				if cellErr != nil {
					return nil, cellErr
				}
				row.AppendChild(fieldValCell)
			}
		}
		if hRowKey != nil {
			headerRows = append(headerRows, hRowKey)
		}

		// cells for values
		if !needsTable(ival.Type()) {
			cell, cellErr := s.simpleTableCell(ival)
			if cellErr != nil {
				return nil, cellErr
			}
			row.AppendChild(cell)
		} else if ival.Kind() == reflect.Struct {
			fields := reflect.VisibleFields(ival.Type())
			var hRowVal *html.Node
			if len(headerRows) > 0 {
				hRowVal = headerRows[0]
			} else {
				hRowVal = createElemAtom(atom.Tr)
			}
			for _, field := range fields {
				if !field.IsExported() {
					// skip the unexported fields for now
					continue
				}
				// TODO: pull out into a helper
				if v, ok := field.Tag.Lookup("statuspage"); ok && v == "-" {
					// The caller asked us to skip this
					continue
				}
				if needsTable(field.Type) {
					ns, genErr := s.genValSection(ival.FieldByIndex(field.Index))
					if genErr != nil {
						return nil, genErr
					}
					for _, n := range ns {
						cell := createElemAtom(atom.Td)
						row.AppendChild(cell)
						cell.AppendChild(n)
					}
				}
				th := createElemAtom(atom.Th)
				hRowVal.AppendChild(th)
				th.AppendChild(textNode(field.Name))

				if needsTable(field.Type) {
					continue
				}

				// add the field values from this struct to the values row
				fieldValCell, cellErr := s.simpleTableCell(ival.FieldByName(field.Name))
				if cellErr != nil {
					return nil, cellErr
				}
				row.AppendChild(fieldValCell)
			}
		} else if ival.Kind() == reflect.Slice || ival.Kind() == reflect.Array {
			if ival.Kind() != reflect.Array && ival.IsNil() {
				nilCell, cErr := s.simpleTableCell(ival)
				if cErr != nil {
					return nil, cErr
				}
				row.AppendChild(nilCell)
				continue
			}
			size := min(ival.Len(), maxSliceLen)
			for i := range size {
				sliceVal := ival.Index(i)
				if needsTable(sliceVal.Type()) {
					// TODO
					continue
				}
				c, cellErr := s.simpleTableCell(sliceVal)
				if cellErr != nil {
					return nil, cellErr
				}
				row.AppendChild(c)
			}
		}

		for _, hRow := range headerRows {
			baseTable.AppendChild(hRow)
		}

		baseTable.AppendChild(row)
	}

	return []*html.Node{baseTable}, nil
}
