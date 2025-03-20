package excel_import

import (
	"reflect"

	"github.com/xuri/excelize/v2"
)

type Sheet struct {
	Name      string
	Headers   map[string]Header
	Values    []map[string]any
	Parent    *Sheet
	SubSheets map[string]*Sheet
}

type Header struct {
	Name  string
	Index int
}

func (h *Header) Cell(row int) string {
	column, err := excelize.CoordinatesToCellName(h.Index, row)
	if err != nil {
		panic(err)
	}
	return column
}

func ToSheet(model any, prefix string, sheet *Sheet, valueMap map[string]any, parent *Sheet) (*Sheet, map[string]any) {
	if sheet == nil {
		sheet = new(Sheet)
		sheet.Name = reflect.TypeOf(model).Name()
		sheet.Parent = parent
		sheet.SubSheets = make(map[string]*Sheet)
		sheet.Headers = make(map[string]Header)
		sheet.Values = make([]map[string]any, 0)
	}

	if valueMap == nil {
		valueMap = make(map[string]any)
	}

	t := reflect.TypeOf(model)
	v := reflect.ValueOf(model)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
		v = v.Elem()
	}

	for i := range t.NumField() {
		field := t.Field(i)
		fieldValue := v.Field(i)

		if field.Anonymous {
			// Process embedded fields into the same valueMap without appending to Values
			sheet, _ = ToSheet(fieldValue.Interface(), prefix, sheet, valueMap, sheet)
			continue
		}

		tag := field.Tag.Get("csv")
		if tag == "" || tag == "-" {
			continue
		}

		if field.Type.Kind() == reflect.Ptr && field.Type.Elem().Kind() == reflect.Struct {
			if fieldValue.IsNil() {
				continue // Skip nil struct pointers
			}
			internalPrefix := tag
			if prefix != "" {
				internalPrefix = prefix + "." + tag
			}
			sheet, _ = ToSheet(fieldValue.Interface(), internalPrefix, sheet, valueMap, sheet)
		} else if field.Type.Kind() == reflect.Struct {
			internalPrefix := tag
			if prefix != "" {
				internalPrefix = prefix + "." + tag
			}
			sheet, _ = ToSheet(fieldValue.Interface(), internalPrefix, sheet, valueMap, sheet)
		} else if field.Type.Kind() == reflect.Slice {
			var subSheet *Sheet
			subSheetName := reflect.TypeOf(model).Field(i).Type.Elem().Name()
			_, ok := sheet.SubSheets[subSheetName]
			if ok {
				subSheet = sheet.SubSheets[subSheetName]
			}
			internalPrefix := tag
			if prefix != "" {
				internalPrefix = prefix + "." + tag
			}
			for j := range fieldValue.Len() {
				subSheet, _ = ToSheet(fieldValue.Index(j).Interface(), internalPrefix, subSheet, nil, sheet)
			}
			sheet.SubSheets[subSheet.Name] = subSheet
		} else {
			internalPrefix := tag
			if prefix != "" {
				internalPrefix = prefix + "." + tag
			}
			if _, ok := sheet.Headers[internalPrefix]; !ok {
				sheet.Headers[internalPrefix] = Header{
					Name:  internalPrefix,
					Index: len(sheet.Headers) + 1,
				}
			}

			if fieldValue.Kind() == reflect.Ptr {
				if !fieldValue.IsNil() {
					valueMap[internalPrefix] = fieldValue.Elem().Interface()
				}
			} else {
				valueMap[internalPrefix] = fieldValue.Interface()
			}
		}
	}

	// Only append to Values at the top level (when parent is nil)
	if parent == nil && len(valueMap) > 0 {
		sheet.Values = append(sheet.Values, valueMap)
	}

	return sheet, valueMap
}
