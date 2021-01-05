package configcat

import (
	"reflect"
	"sort"
)

// visibleFields returns all the visible fields in t, which must be a
// struct type. A field is defined as visible if it's accessible
// directly with a FieldByName call. The returned fields include fields
// inside anonymous struct members and unexported fields. They follow
// the same order found in the struct, with anonymous fields followed
// immediately by their promoted fields.
//
// For each element e of the returned slice, the corresponding field
// can be retrieved from a value v of type t by calling v.FieldByIndex(e.Index).
//
// If this makes it into the standard library, we could remove it...
// https://github.com/golang/go/issues/42782
func visibleFields(t reflect.Type) []reflect.StructField {
	byName := make(map[string]reflect.StructField)
	addFields(t, byName, nil)
	fields := make(fieldsByIndex, 0, len(byName))
	for _, f := range byName {
		if f.Name != "" {
			fields = append(fields, f)
		}
	}
	sort.Sort(fields)
	return fields
}

func addFields(t reflect.Type, byName map[string]reflect.StructField, index []int) {
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		index := append(index, i)
		var add bool
		old, ok := byName[f.Name]
		switch {
		case ok && len(old.Index) == len(index):
			// Fields with the same name at the same depth
			// cancel one another out. Set the field name
			// to empty to signify that has happened.
			old.Name = ""
			byName[f.Name] = old
			add = false
		case ok:
			// Fields at less depth win.
			add = len(index) < len(old.Index)
		default:
			// The field did not previously exist.
			add = true
		}
		if add {
			// copy the index so that it's not overwritten
			// by the other appends.
			f.Index = append([]int(nil), index...)
			byName[f.Name] = f
		}
		if f.Anonymous {
			if f.Type.Kind() == reflect.Ptr {
				f.Type = f.Type.Elem()
			}
			if f.Type.Kind() == reflect.Struct {
				addFields(f.Type, byName, index)
			}
		}
	}
}

type fieldsByIndex []reflect.StructField

func (f fieldsByIndex) Len() int {
	return len(f)
}

func (f fieldsByIndex) Swap(i, j int) {
	f[i], f[j] = f[j], f[i]
}

func (f fieldsByIndex) Less(i, j int) bool {
	indexi, indexj := f[i].Index, f[j].Index
	for len(indexi) != 0 && len(indexj) != 0 {
		ii, ij := indexi[0], indexj[0]
		if ii != ij {
			return ii < ij
		}
		indexi, indexj = indexi[1:], indexj[1:]
	}
	return len(indexi) < len(indexj)
}
