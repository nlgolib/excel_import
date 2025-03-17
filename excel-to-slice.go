package excel_import

import (
	"bytes"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

// For now, this function only works with the first sheet in the excel file
// TODO: Make it work with multiple sheets
func ToSlice[T any](data []byte) ([]T, error) {
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, fmt.Errorf("no sheets found in excel file")
	}

	// Parse all sheets first
	sheetsData := make(map[string][]map[string]string)
	for _, sheetName := range sheets {
		rows, err := f.GetRows(sheetName)
		if err != nil {
			return nil, err
		}

		if len(rows) < 2 { // Need at least header row and one data row
			continue
		}

		headers := rows[0]
		var rowsData []map[string]string

		// Process each data row
		for i := 1; i < len(rows); i++ {
			row := rows[i]
			rowData := make(map[string]string)
			for j, val := range row {
				if j < len(headers) && headers[j] != "" {
					rowData[headers[j]] = val
				}
			}
			// Only add non-empty rows
			if len(rowData) > 0 {
				rowsData = append(rowsData, rowData)
			}
		}

		sheetsData[sheetName] = rowsData
	}

	// Identify the main sheet based on type T
	t := reflect.TypeOf(*new(T))
	mainSheetName := t.Name()

	// Find main sheet in the Excel file
	mainRows, ok := sheetsData[mainSheetName]
	if !ok {
		return nil, fmt.Errorf("main sheet with name %s not found", mainSheetName)
	}

	// Build relationship map between sheets
	relationshipMap := buildRelationshipMap(sheetsData)

	// Create result slice
	var result []T

	// Process main objects
	for _, rowData := range mainRows {
		item := new(T)
		v := reflect.ValueOf(item).Elem()

		// Fill the object with data
		if err := populateStruct(v, rowData, sheetsData, relationshipMap, ""); err != nil {
			return nil, err
		}

		result = append(result, *item)
	}

	return result, nil
}

// buildRelationshipMap analyzes sheet names to find parent-child relationships
// For example: "User_Address" indicates Address is a child of User
func buildRelationshipMap(sheetsData map[string][]map[string]string) map[string][]string {
	relationships := make(map[string][]string)

	for sheetName := range sheetsData {
		parts := strings.Split(sheetName, "_")
		if len(parts) > 1 {
			parent := parts[0]
			if _, exists := relationships[parent]; !exists {
				relationships[parent] = []string{}
			}
			relationships[parent] = append(relationships[parent], sheetName)
		}
	}

	return relationships
}

func populateStruct(v reflect.Value, rowData map[string]string, sheetsData map[string][]map[string]string,
	relationshipMap map[string][]string, parentID string) error {
	t := v.Type()

	// First pass: set basic fields and capture ID
	var id string
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		tag := field.Tag.Get("csv")
		if tag == "" || tag == "-" {
			continue
		}

		// Handle basic types
		if fieldValue, exists := rowData[tag]; exists {
			if tag == "ID" || strings.HasSuffix(tag, "ID") {
				id = fieldValue
			}
			setValue(v.Field(i), fieldValue)
		}
	}

	// Second pass: handle complex types (structs, slices)
	typeName := t.Name()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fieldValue := v.Field(i)

		if !fieldValue.CanSet() {
			continue
		}

		switch field.Type.Kind() {
		case reflect.Struct:
			// Handle embedded struct
			handleEmbeddedStruct(fieldValue, rowData, sheetsData, relationshipMap, id)

		case reflect.Ptr:
			if field.Type.Elem().Kind() == reflect.Struct {
				// Handle pointer to struct
				childType := field.Type.Elem().Name()
				childSheetName := fmt.Sprintf("%s_%s", typeName, childType)

				if childRows, exists := sheetsData[childSheetName]; exists {
					for _, childRow := range childRows {
						if parentField, exists := childRow["ParentID"]; exists && parentField == id {
							newStruct := reflect.New(field.Type.Elem()).Elem()
							if err := populateStruct(newStruct, childRow, sheetsData, relationshipMap, id); err != nil {
								return err
							}
							fieldValue.Set(newStruct.Addr())
							break
						}
					}
				}
			}

		case reflect.Slice:
			if field.Type.Elem().Kind() == reflect.Struct ||
				(field.Type.Elem().Kind() == reflect.Ptr && field.Type.Elem().Elem().Kind() == reflect.Struct) {
				// Handle slice of structs or pointers to structs
				var elemType reflect.Type
				isPtr := field.Type.Elem().Kind() == reflect.Ptr
				if isPtr {
					elemType = field.Type.Elem().Elem()
				} else {
					elemType = field.Type.Elem()
				}

				childType := elemType.Name()
				childSheetName := fmt.Sprintf("%s_%s", typeName, childType)

				if childRows, exists := sheetsData[childSheetName]; exists {
					slice := reflect.MakeSlice(field.Type, 0, 0)

					for _, childRow := range childRows {
						if parentField, exists := childRow["ParentID"]; exists && parentField == id {
							var elem reflect.Value
							if isPtr {
								elem = reflect.New(elemType)
								if err := populateStruct(elem.Elem(), childRow, sheetsData, relationshipMap, id); err != nil {
									return err
								}
							} else {
								elem = reflect.New(elemType).Elem()
								if err := populateStruct(elem, childRow, sheetsData, relationshipMap, id); err != nil {
									return err
								}
							}
							slice = reflect.Append(slice, elem)
						}
					}

					if slice.Len() > 0 {
						fieldValue.Set(slice)
					}
				}
			}
		}
	}

	return nil
}

func handleEmbeddedStruct(v reflect.Value, rowData map[string]string, sheetsData map[string][]map[string]string,
	relationshipMap map[string][]string, parentID string) error {
	t := v.Type()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		tag := field.Tag.Get("csv")
		if tag == "" || tag == "-" {
			continue
		}

		if fieldValue, exists := rowData[tag]; exists {
			setValue(v.Field(i), fieldValue)
		}
	}

	return nil
}

func setValue(field reflect.Value, value string) {
	if !field.CanSet() || value == "" {
		return
	}

	switch field.Kind() {
	case reflect.String:
		field.SetString(value)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if v, err := strconv.ParseInt(value, 10, 64); err == nil {
			field.SetInt(v)
		}
	case reflect.Float32, reflect.Float64:
		if v, err := strconv.ParseFloat(value, 64); err == nil {
			field.SetFloat(v)
		}
	case reflect.Bool:
		if v, err := strconv.ParseBool(value); err == nil {
			field.SetBool(v)
		}
	}
}
