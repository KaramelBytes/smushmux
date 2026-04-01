package parser

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
)

func TestMarkdownParserNormalizesLineEndingsAndBlankLines(t *testing.T) {
	p := markdownParser{}
	in := []byte("# Title\r\n\r\n\r\nBody\rLine2\n\n\n\nEnd")

	out, err := p.Parse(in)
	if err != nil {
		t.Fatalf("markdown parse: %v", err)
	}
	if strings.Contains(out, "\r") {
		t.Fatalf("expected CRs normalized out: %q", out)
	}
	if strings.Contains(out, "\n\n\n") {
		t.Fatalf("expected blank lines collapsed: %q", out)
	}
}

func TestTxtParserRoundTrip(t *testing.T) {
	p := txtParser{}
	in := []byte("a\n\n b\t c")
	out, err := p.Parse(in)
	if err != nil {
		t.Fatalf("txt parse: %v", err)
	}
	if out != string(in) {
		t.Fatalf("txt parse should preserve bytes as string: got %q", out)
	}
}

func TestDocxParserInvalidZip(t *testing.T) {
	p := docxParser{}
	if _, err := p.Parse([]byte("not-a-zip")); err == nil {
		t.Fatalf("expected open docx error for invalid zip")
	}
}

func TestDocxParserMissingDocumentXML(t *testing.T) {
	p := docxParser{}
	var b bytes.Buffer
	zw := zip.NewWriter(&b)
	if _, err := zw.Create("word/other.xml"); err != nil {
		t.Fatalf("create zip entry: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}

	if _, err := p.Parse(b.Bytes()); err == nil || !strings.Contains(err.Error(), "document.xml not found") {
		t.Fatalf("expected missing document.xml error, got: %v", err)
	}
}

func TestDocxParserExtractsText(t *testing.T) {
	p := docxParser{}
	var b bytes.Buffer
	zw := zip.NewWriter(&b)
	w, err := zw.Create("word/document.xml")
	if err != nil {
		t.Fatalf("create document.xml: %v", err)
	}
	xml := `<w:document><w:body><w:p><w:r><w:t>Hello</w:t></w:r></w:p><w:p><w:r><w:t>World</w:t></w:r></w:p></w:body></w:document>`
	if _, err := w.Write([]byte(xml)); err != nil {
		t.Fatalf("write document.xml: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}

	out, err := p.Parse(b.Bytes())
	if err != nil {
		t.Fatalf("docx parse: %v", err)
	}
	if !strings.Contains(out, "Hello") || !strings.Contains(out, "World") {
		t.Fatalf("expected extracted text to contain words, got %q", out)
	}
}

func TestParseXLSXFileMissingPath(t *testing.T) {
	if _, err := ParseXLSXFile("/definitely/not/there.xlsx", "", 0); err == nil {
		t.Fatalf("expected error for missing xlsx file")
	}
}
