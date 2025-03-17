package excel_import

import (
	"github.com/xuri/excelize/v2"
)

func ToExcel[T any](slice []T) ([]byte, error) {

	f := excelize.NewFile()

	var sheet *Sheet

	for _, model := range slice {
		sheet, _ = ToSheet(model, "", sheet, nil, nil)
	}

	writeSheet(sheet, f)

	f.DeleteSheet("Sheet1")
	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeSheet(sheet *Sheet, f *excelize.File) {
	f.NewSheet(sheet.Name)
	for _, header := range sheet.Headers {
		f.SetCellValue(sheet.Name, header.Cell(1), header.Name)
	}
	row := 2
	for _, value := range sheet.Values {
		for _, header := range sheet.Headers {
			f.SetCellValue(sheet.Name, header.Cell(row), value[header.Name])
		}
		row++
	}
	for _, subSheet := range sheet.SubSheets {
		writeSheet(subSheet, f)
	}
}
