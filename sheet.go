package excel

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

	t := reflect.TypeOf(model)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	if valueMap == nil {
		valueMap = make(map[string]any)
	}

	for i := range t.NumField() {
		field := t.Field(i)
		tag := field.Tag.Get("excel")
		if tag == "" || tag == "-" {
			continue
		}

		if field.Anonymous {
			sheet, valueMap = ToSheet(reflect.ValueOf(model).Field(i).Interface(), "", sheet, valueMap, nil)
		} else if field.Type.Kind() == reflect.Ptr && field.Type.Elem().Kind() == reflect.Struct {
			internalPrefix := tag
			if prefix != "" {
				internalPrefix = prefix + "." + tag
			}
			sheet, valueMap = ToSheet(reflect.ValueOf(model).Field(i).Interface(), internalPrefix, sheet, valueMap, nil)
		} else if field.Type.Kind() == reflect.Struct {
			internalPrefix := tag
			if prefix != "" {
				internalPrefix = prefix + "." + tag
			}
			sheet, valueMap = ToSheet(reflect.ValueOf(model).Field(i).Interface(), internalPrefix, sheet, valueMap, nil)
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
			for j := range reflect.ValueOf(model).Field(i).Len() {
				subSheet, _ = ToSheet(reflect.ValueOf(model).Field(i).Index(j).Interface(), internalPrefix, subSheet, nil, sheet)
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

			var v reflect.Value
			if reflect.TypeOf(model).Kind() == reflect.Ptr {
				v = reflect.ValueOf(model).Elem().Field(i)
			} else {
				v = reflect.ValueOf(model).Field(i)
			}
			if v.Kind() == reflect.Ptr {
				v = v.Elem()
			}
			valueMap[internalPrefix] = v.Interface()
		}
	}
	if len(valueMap) > 0 {
		sheet.Values = append(sheet.Values, valueMap)
	}
	return sheet, nil
}
