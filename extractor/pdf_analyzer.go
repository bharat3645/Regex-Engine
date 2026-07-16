// File: extractor/pdf_analyzer.go
package extractor

import (
	"github.com/ledongthuc/pdf"
)

// isImageBasedPDF checks a PDF to determine if it's primarily
// composed of images rather than selectable text.
func isImageBasedPDF(path string) (bool, error) {
	f, r, err := pdf.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	totalTextLen := 0
	pagesToCheck := r.NumPage()
	if pagesToCheck > 3 {
		pagesToCheck = 3 // Check first 3 pages only for performance
	}

	for i := 1; i <= pagesToCheck; i++ {
		p := r.Page(i)
		if p.V.IsNull() {
			continue
		}

		// Extract raw text from the page
		content, err := p.GetPlainText(nil)
		if err != nil {
			continue
		}
		totalTextLen += len(content)
	}

	// Heuristic: If we have very little text for multiple pages, assume it's image-based.
	// Threshold: < 50 characters of text per page average.
	if pagesToCheck > 0 && (totalTextLen/pagesToCheck) < 50 {
		return true, nil
	}

	return false, nil
}
