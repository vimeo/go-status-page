package statuspage

import (
	"fmt"
	"reflect"
	"strconv"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

func sliceArrayValScalar(et reflect.Type) bool {
	if et.Implements(stringerReflectType) {
		return true
	}
	switch et.Kind() {
	case reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.UnsafePointer,
		reflect.Float32, reflect.Float64,
		reflect.Complex64, reflect.Complex128,
		reflect.String,
		reflect.Chan:
		return true
	case reflect.Array:
		// size 0  arrays are scalar
		return et.Len() == 0
	case reflect.Slice:
		return false
	case reflect.Map:
		// TODO: switch to false and generate linky things
		return true
	case reflect.Struct:
		// The empty struct is scalar :)
		return et.NumField() < 1
	case reflect.Interface:
		// This will be fun: we'll have to check whether all the implementations are scalars, structs, etc.
		return false
	case reflect.Pointer:
		// strip off a layer of pointers
		return sliceArrayValScalar(et.Elem())
	case reflect.Func:
		// TODO: swtich to et.CanSeq() || et.CanSeq2() and generate linky things
		return true
	default:
		panic(fmt.Errorf("unhandled element kind: %s type %s", et.Kind(), et))
	}
}

func (s *Status[T]) genSliceArrayTable(v reflect.Value) ([]*html.Node, error) {
	tbl := (*html.Node)(nil)
	capNode := createElemAtom(atom.Caption)
	capNode.AppendChild(textNode(v.Type().String()))
	switch v.Kind() {
	case reflect.Array:
		switch v.Type().Len() {
		case 0:
			// nothing more to say
			return []*html.Node{capNode}, nil
		default:
		}
	case reflect.Slice:
		if v.IsNil() {
			return []*html.Node{textNode(v.Type().String() + "(nil)")}, nil
		}
		capNode.AppendChild(createElemAtom(atom.Br))
		capNode.AppendChild(textNode("len() = " + strconv.Itoa(v.Len())))
		capNode.AppendChild(createElemAtom(atom.Br))
		capNode.AppendChild(textNode("cap() = " + strconv.Itoa(v.Cap())))
	case reflect.Func:
		capNode.AppendChild(createElemAtom(atom.Br))
		if v.Type().CanSeq2() {
			capNode.AppendChild(textNode("iter.Seq: (" + v.Type().In(0).In(0).String() + ", " + v.Type().In(0).In(1).String() + ")"))
		} else if v.Type().CanSeq() {
			capNode.AppendChild(textNode("iter.Seq: " + v.Type().In(0).In(0).String()))
		} else {
			panic("non-iter.Seq2? passed to genSliceArrayTable: " + v.Type().String())
		}
	default:
		panic(fmt.Errorf("non-slice/array kind: %s type %s", v.Kind(), v.Type()))
	}

	if sliceArrayValScalar(v.Type().Elem()) {
		sNode, sErr := s.scalarSliceArrayTable(v)
		if sErr != nil {
			return nil, fmt.Errorf("failed to generate table for slice/array of type %s: %w", v.Type(), sErr)
		}
		tbl = sNode
		tbl.InsertBefore(capNode, tbl.FirstChild)
		return []*html.Node{tbl}, nil
	}
	elemType := v.Type().Elem()
	switch elemType.Kind() {
	case reflect.Map:
		// We'll punt and put a table in a table for now. (we'll generate links to pages per-map later)
		panic("map element-type not handled as scalar array 🤷 (should have been handled in sliceArrayValScalar)")
	case reflect.Struct, reflect.Pointer:
		stNode, stErr := s.structSliceArrayTable(v)
		if stErr != nil {
			return nil, fmt.Errorf("failed to generate table for slice/array of type %s: %w", v.Type(), stErr)
		}
		tbl = stNode
	case reflect.Array, reflect.Slice:
		slNode, slErr := s.sliceArraySliceValTable(v)
		if slErr != nil {
			return nil, fmt.Errorf("failed to generate table for slice/array of type %s: %w", v.Type(), slErr)
		}
		tbl = slNode
	case reflect.Func:
		// We're only here if the function is iterable
		panic("iterator element-type not handled as scalar array 🤷 (should have been handled in sliceArrayValScalar)")
	case reflect.Interface:
		// This will be fun: we'll have to check whether all the implementations are scalars, structs, etc.
		elemT, uniform := allIfaceSliceElemsSame(v)
		if !uniform {
			// Just put tables inside tables. It's ugly, but for now, it's not the worst thing we can do
			stNode, stErr := s.scalarSliceArrayTable(v)
			if stErr != nil {
				return nil, fmt.Errorf("failed to generate table for slice/array of type %s: %w", v.Type(), stErr)
			}
			tbl = stNode
		} else {
			stNode, stErr := s.ifaceSliceArrayTable(v, elemT)
			if stErr != nil {
				return nil, fmt.Errorf("failed to generate table for slice/array of type %s: %w", v.Type(), stErr)
			}
			tbl = stNode
		}
	}
	// add the caption we created at the top (it must be the first child of the table)
	// Fortunately, InsertBefore handles a nil `oldChild` arg as a request to append to the end, so the empty table
	// case should work properly.
	tbl.InsertBefore(capNode, tbl.FirstChild)

	return []*html.Node{tbl}, nil
}

func (s *Status[T]) scalarSliceArrayTable(v reflect.Value) (*html.Node, error) {
	// One-column table for this slice, array or iter.Seq
	tbl := createElemAtom(atom.Table)
	offset := 0
	for ev := range v.Seq() {
		row := createElemAtom(atom.Tr)
		tbl.AppendChild(row)
		e := createElemAtom(atom.Td)
		row.AppendChild(e)
		// since we're working with a scalar-ish value, we can append children for all return values from genValSection here.
		ns, rendErr := s.genValSection(ev)
		if rendErr != nil {
			return nil, fmt.Errorf("failed to render table element at index %d in slice/array of type %s: %w",
				offset, v.Type(), rendErr)
		}
		for _, n := range ns {
			e.AppendChild(n)
		}
		offset++
	}
	return tbl, nil
}

func arraySliceStructHeaderRow(t reflect.Type) (*html.Node, int, error) {
	if t.Kind() == reflect.Pointer {
		return arraySliceStructHeaderRow(t.Elem())
	}
	if t.Kind() != reflect.Struct {
		panic(fmt.Errorf("non-struct type passed: %s", t))
	}
	row := createElemAtom(atom.Tr)
	fs := reflect.VisibleFields(t)
	nCols := 0
	for _, fs := range fs {
		if shouldSkipField(fs) {
			continue
		}
		h := createElemAtom(atom.Th)
		row.AppendChild(h)
		h.Attr = []html.Attribute{{Key: atom.Alt.String(), Val: fs.Type.String()}}
		h.AppendChild(textNode(fs.Name))
		nCols++
	}
	return row, nCols, nil
}

// iterates over an array or slice, and returns a type+true if all elements are the one type or nil
func allIfaceSliceElemsSame(v reflect.Value) (reflect.Type, bool) {
	t := reflect.Type(nil)
	for z := 0; z < v.Len(); z++ {
		iv := v.Index(z)
		if iv.IsNil() {
			// interface has nil-type
			continue
		}
		// interfaces can't contain other interface-types directly, so there's no need to
		// iteratively/recursively unwrap here.
		// Since types (including pointer-types) are uniquely comparable, there's no need to unwrap any further
		// here.
		// For now, since it's a super-annoying case to check we'll skip recursive unwrapping to check that
		// there isn't a pointer to an interface buried in there.

		if t == nil {
			t = iv.Elem().Type()
			continue
		}
		if iv.Elem().Type() != t {
			return nil, false
		}
	}
	return t, t != nil
}

func (s *Status[T]) arraySliceStructDataRow(v reflect.Value, nCols int) (*html.Node, error) {
	if v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface {
		if v.IsNil() {
			row := createElemAtom(atom.Tr)
			nilVal := createElemAtom(atom.Td)
			nilVal.Attr = []html.Attribute{{Key: atom.Colspan.String(), Val: strconv.Itoa(nCols)}}
			nilVal.AppendChild(textNode(v.Type().String() + "(nil)"))
			return row, nil
		}
		return s.arraySliceStructDataRow(v.Elem(), nCols)
	}
	if v.Kind() != reflect.Struct {
		panic(fmt.Errorf("non-struct type passed: %s", v.Type()))
	}
	row := createElemAtom(atom.Tr)
	fs := reflect.VisibleFields(v.Type())
	for _, fs := range fs {
		if shouldSkipField(fs) {
			continue
		}
		d := createElemAtom(atom.Td)
		row.AppendChild(d)
		fd := v.FieldByIndex(fs.Index)
		ns, nErr := s.genValSection(fd)
		if nErr != nil {
			return nil, fmt.Errorf("failed to generate element for field %q of type %s: %w",
				fs.Name, fd.Type(), nErr)
		}
		for _, n := range ns {
			d.AppendChild(n)
		}
	}
	return row, nil
}

func (s *Status[T]) structSliceArrayTable(v reflect.Value) (*html.Node, error) {
	tbl := createElemAtom(atom.Table)
	h, nCols, hErr := arraySliceStructHeaderRow(v.Type().Elem())
	if hErr != nil {
		return nil, fmt.Errorf("failed to generate header for type %s: %w", v.Type(), hErr)
	}
	tbl.AppendChild(h)
	offset := 0
	for ev := range v.Seq() {
		dr, drErr := s.arraySliceStructDataRow(ev, nCols)
		if drErr != nil {
			return nil, fmt.Errorf("failed to generate row %d for type %s: %w", offset, v.Type(), drErr)
		}
		tbl.AppendChild(dr)
		// TODO: should we have an index column?
		offset++
	}
	return tbl, nil
}

func (s *Status[T]) ifaceSliceArrayTable(v reflect.Value, uniformType reflect.Type) (*html.Node, error) {
	tbl := createElemAtom(atom.Table)
	h, nCols, hErr := arraySliceStructHeaderRow(uniformType)
	if hErr != nil {
		return nil, fmt.Errorf("failed to generate header for type %s: %w", v.Type(), hErr)
	}
	tbl.AppendChild(h)
	offset := 0
	for ev := range v.Seq() {
		dr, drErr := s.arraySliceStructDataRow(ev, nCols)
		if drErr != nil {
			return nil, fmt.Errorf("failed to generate row %d for type %s: %w", offset, v.Type(), drErr)
		}
		tbl.AppendChild(dr)
		// TODO: should we have an index column?
		offset++
	}
	return tbl, nil
}

// handle two-dimensional arrays/slices
func (s *Status[T]) sliceArraySliceValTable(v reflect.Value) (*html.Node, error) {
	// get the max slice-length
	// TODO: define a max width where we start doing something clever with omitting middle members and generating links to the relevant entries
	maxElemLen := 0
	switch v.Kind() {
	case reflect.Array:
		maxElemLen = v.Type().Len()
	case reflect.Slice:
		for ev := range v.Seq() {
			if ev.IsNil() {
				// nil, keep going
				continue
			}
			maxElemLen = max(ev.Len(), maxElemLen)
		}
	}

	tbl := createElemAtom(atom.Table)
	offset := 0
	// now, we can generate the table
	for ev := range v.Seq() {
		row := createElemAtom(atom.Tr)
		tbl.AppendChild(row)
		if ev.Kind() != reflect.Array {
			// if it's not an array, iteratively unwrap
			for {
				if ev.IsNil() {
					// TODO: include option for offset column and column-headings
					nilVal := createElemAtom(atom.Td)
					nilVal.Attr = []html.Attribute{{Key: atom.Colspan.String(), Val: strconv.Itoa(maxElemLen)}}
					nilVal.AppendChild(textNode(ev.Type().String() + "(nil)"))
					row.AppendChild(nilVal)

					continue
				}
				// do the loop check at the bottom so slices get the nil-check as well :)
				if ev.Kind() == reflect.Array || ev.Kind() == reflect.Slice {
					break
				}
				// it's a pointer (or maybe a nested interface that we've previously determined
				// uniformly unwraps to exactly one slice/array type (or nil)
				// keep unwrapping
				// TODO: add an indicator of how far we've unwrapped (and what types we've gone though)
				ev = ev.Elem()
			}
		}
		colOffset := 0
		for colVal := range ev.Seq() {
			colElem := createElemAtom(atom.Td)
			row.AppendChild(colElem)
			ns, tblCellGenErr := s.genValSection(colVal)
			if tblCellGenErr != nil {
				return nil, fmt.Errorf("failed to generate html for value at offset [%d][%d] in array/slice of type %s: %w",
					offset, colOffset, v.Type(), tblCellGenErr)
			}
			for _, n := range ns {
				colElem.AppendChild(n)
			}
			colOffset++
		}
		offset++
	}
	return tbl, nil
}
