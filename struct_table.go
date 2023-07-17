package statuspage

import (
	"fmt"
	"reflect"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

func needsTable(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Map, reflect.Array, reflect.Slice, reflect.Struct:
		return true
	case reflect.Pointer:
		return needsTable(t.Elem())
	default:
		return false
	}
}

func (s *Status[T]) genStructTable(v reflect.Value) ([]*html.Node, error) {
	if v.Kind() != reflect.Struct {
		panic(fmt.Errorf("non-struct kind: %s type %s", v.Kind(), v.Type()))
	}
	vt := v.Type()

	// get all the fields visible at the top-level. We'll split them into
	// simple fields that can be dropped into a table at the top, and
	// tableFields that need their own tables.
	fields := reflect.VisibleFields(vt)
	simpleFields := make([]reflect.StructField, 0, len(fields))
	tableFields := make([]reflect.StructField, 0, len(fields))
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
			tableFields = append(tableFields, field)
			continue
		}
		simpleFields = append(simpleFields, field)
	}

	// TODO: include the type-name at the top
	out := make([]*html.Node, 0, len(tableFields)+1)
	if len(simpleFields) > 0 {
		simpleTable := createElemAtom(atom.Table)
		out = append(out, simpleTable)

		// iterate over the simple fields and add rows for each row.
		for _, sf := range simpleFields {
			row := createElemAtom(atom.Tr)
			simpleTable.AppendChild(row)
			fieldCol := createElemAtom(atom.Td)
			fieldCol.AppendChild(textNode(sf.Name))
			row.AppendChild(fieldCol)

			valCol := createElemAtom(atom.Td)
			row.AppendChild(valCol)
			sv := v.FieldByName(sf.Name)
			// We've already validated that this is a simple-enough type, so use
			// genValSection to render into a (small number of?) nodes
			valNs, valSectionErr := s.genValSection(sv)
			if valSectionErr != nil {
				return nil, fmt.Errorf("failed to render field %q: %w", sf.Name, valSectionErr)
			}
			for _, valN := range valNs {
				valCol.AppendChild(valN)
			}
		}
	}

	// iterate over the remaining table fields and generate sections for each field (with their
	// own tables)
	// they'll each delegate to genValSection() to create the fields.
	for _, tf := range tableFields {
		section := createElemAtom(atom.Div)
		out = append(out, section)
		fieldName := createElemAtom(atom.H3)
		fieldName.AppendChild(textNode(tf.Name))
		section.AppendChild(fieldName)
		section.AppendChild(createElemAtom(atom.Br))

		sv := v.FieldByName(tf.Name)
		valNs, valSectionErr := s.genValSection(sv)
		if valSectionErr != nil {
			return nil, fmt.Errorf("failed to render field %q: %w", tf.Name, valSectionErr)
		}
		for _, valN := range valNs {
			section.AppendChild(valN)
		}
	}

	return out, nil
}
