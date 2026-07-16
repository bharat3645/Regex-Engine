// File: extractor/native_handlers.go
package extractor

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ledongthuc/pdf"
	"github.com/nguyenthenguyen/docx"
	"github.com/xuri/excelize/v2"
)

// extractTextFromDocx reads a .docx file and returns its text content.
func extractTextFromDocx(ctx context.Context, path string) (io.ReadCloser, error) {
	r, err := docx.ReadDocxFile(path)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	content := r.Editable().GetContent()
	return io.NopCloser(strings.NewReader(content)), nil
}

// extractTextFromXlsx reads an .xlsx file and returns its text content.
func extractTextFromXlsx(ctx context.Context, path string) (io.ReadCloser, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var content strings.Builder
	sheets := f.GetSheetList()
	for _, sheet := range sheets {
		rows, err := f.GetRows(sheet)
		if err != nil {
			// Log this error but continue to the next sheet
			continue
		}
		for _, row := range rows {
			for _, colCell := range row {
				fmt.Fprint(&content, colCell, " ")
			}
			fmt.Fprintln(&content)
		}
	}
	return io.NopCloser(strings.NewReader(content.String())), nil
}

// extractTextFromPdf reads a .pdf file and returns its text content.
// Note: This works best for text-based PDFs, not scanned images.
func extractTextFromPdf(ctx context.Context, path string) (io.ReadCloser, error) {
	f, r, err := pdf.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var content strings.Builder
	totalPage := r.NumPage()

	for pageIndex := 1; pageIndex <= totalPage; pageIndex++ {
		p := r.Page(pageIndex)
		if p.V.IsNull() {
			continue
		}
		text, err := p.GetPlainText(nil)
		if err != nil {
			return nil, err
		}
		fmt.Fprintln(&content, text)
	}
	return io.NopCloser(strings.NewReader(content.String())), nil
}

// extractPlainText opens a file and returns its reader directly.
func extractPlainText(ctx context.Context, path string) (io.ReadCloser, error) {
	return os.Open(path)
}
