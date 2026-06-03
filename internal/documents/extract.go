// Package documents handles ingestion of job descriptions and resumes:
// extracting plain text from PDF/DOCX/TXT uploads (or pasted text) and then
// using the LLM to structure that text into domain types.
package documents

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/dslipak/pdf"
)

// ExtractText returns plain text from an uploaded document. The format is
// inferred from the filename extension; unknown/empty extensions are treated as
// plain text. Use this for PDF, DOCX, and TXT/MD uploads.
func ExtractText(filename string, data []byte) (string, error) {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".pdf":
		return extractPDF(data)
	case ".docx":
		return extractDOCX(data)
	case ".txt", ".md", "":
		return string(data), nil
	default:
		// Best-effort: assume it is already text.
		return string(data), nil
	}
}

func extractPDF(data []byte) (string, error) {
	r, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("open pdf: %w", err)
	}
	var sb strings.Builder
	totalPage := r.NumPage()
	for i := 1; i <= totalPage; i++ {
		page := r.Page(i)
		if page.V.IsNull() {
			continue
		}
		text, err := page.GetPlainText(nil)
		if err != nil {
			return "", fmt.Errorf("read pdf page %d: %w", i, err)
		}
		sb.WriteString(text)
		sb.WriteString("\n")
	}
	return strings.TrimSpace(sb.String()), nil
}

var (
	docxParaClose = regexp.MustCompile(`</w:p>`)
	docxTabRun    = regexp.MustCompile(`<w:tab\b[^>]*/>`)
	docxBreakRun  = regexp.MustCompile(`<w:br\b[^>]*/>`)
	xmlTag        = regexp.MustCompile(`<[^>]+>`)
)

// extractDOCX reads word/document.xml from the .docx zip and strips XML,
// preserving paragraph and tab boundaries. Stdlib-only, no external dependency.
func extractDOCX(data []byte) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("open docx zip: %w", err)
	}
	for _, f := range zr.File {
		if f.Name != "word/document.xml" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return "", fmt.Errorf("open document.xml: %w", err)
		}
		raw, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return "", fmt.Errorf("read document.xml: %w", err)
		}
		s := string(raw)
		s = docxParaClose.ReplaceAllString(s, "\n")
		s = docxTabRun.ReplaceAllString(s, "\t")
		s = docxBreakRun.ReplaceAllString(s, "\n")
		s = xmlTag.ReplaceAllString(s, "")
		s = unescapeXML(s)
		return strings.TrimSpace(s), nil
	}
	return "", fmt.Errorf("docx: word/document.xml not found")
}

func unescapeXML(s string) string {
	replacer := strings.NewReplacer(
		"&amp;", "&",
		"&lt;", "<",
		"&gt;", ">",
		"&quot;", `"`,
		"&apos;", "'",
	)
	return replacer.Replace(s)
}
