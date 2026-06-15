package main

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDocxHTMLPreview(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "styled.docx")
	documentXML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p><w:pPr><w:pStyle w:val="Heading1"/></w:pPr><w:r><w:t>Title</w:t></w:r></w:p>
    <w:p><w:r><w:rPr><w:b/></w:rPr><w:t>Bold line</w:t></w:r></w:p>
  </w:body>
</w:document>`
	if err := writeRawDocx(path, documentXML); err != nil {
		t.Fatal(err)
	}
	html, err := readDocxHTML(path)
	if err != nil {
		t.Fatalf("html: %v", err)
	}
	if !strings.Contains(html, "<h1>") || !strings.Contains(html, "Title") {
		t.Fatalf("missing heading html: %s", html)
	}
	if !strings.Contains(html, "<strong>") || !strings.Contains(html, "Bold") || !strings.Contains(html, "line") {
		t.Fatalf("missing bold html: %s", html)
	}
}

func TestDocxRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.docx")
	original := "标题一行\n\n第二段带 **markdown** 样式也会当纯文本保存。"
	if err := writeDocxPlainText(path, original); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := readDocxPlainText(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if strings.TrimSpace(got) != strings.TrimSpace(original) {
		t.Fatalf("round trip mismatch:\nwant %q\ngot %q", original, got)
	}
}

func TestDocxSkipsFieldInstructions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fields.docx")
	documentXML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p>
      <w:r><w:fldChar w:fldCharType="begin"/></w:r>
      <w:r><w:instrText> TOC \o "1-3" </w:instrText></w:r>
      <w:r><w:fldChar w:fldCharType="separate"/></w:r>
      <w:r><w:t>摘要</w:t></w:r>
      <w:r><w:fldChar w:fldCharType="end"/></w:r>
    </w:p>
  </w:body>
</w:document>`
	if err := writeRawDocx(path, documentXML); err != nil {
		t.Fatal(err)
	}
	text, err := readDocxPlainText(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if strings.Contains(text, "TOC") || strings.Contains(text, "instrText") {
		t.Fatalf("field instruction leaked: %q", text)
	}
	if !strings.Contains(text, "摘要") {
		t.Fatalf("missing display text: %q", text)
	}
	html, err := readDocxHTML(path)
	if err != nil {
		t.Fatalf("html: %v", err)
	}
	if strings.Contains(html, "TOC") {
		t.Fatalf("field instruction leaked into html: %s", html)
	}
	if !strings.Contains(html, "摘要") {
		t.Fatalf("missing display text in html: %s", html)
	}
}

func TestReadWriteDocumentLegacyDocWithoutWord(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "legacy.doc")
	if err := os.WriteFile(path, []byte("not a real doc"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := readWriteDocument(path)
	if err == nil {
		t.Fatal("expected legacy doc read to fail without Word installed")
	}
}

func TestDocxPreviewTableAndLatin(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "table.docx")
	documentXML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:tbl>
      <w:tr>
        <w:tc><w:p><w:r><w:t>中文</w:t></w:r></w:p></w:tc>
        <w:tc><w:p><w:r><w:t>English</w:t></w:r></w:p></w:tc>
      </w:tr>
    </w:tbl>
  </w:body>
</w:document>`
	if err := writeRawDocx(path, documentXML); err != nil {
		t.Fatal(err)
	}
	html, err := readDocxHTML(path)
	if err != nil {
		t.Fatalf("html: %v", err)
	}
	if !strings.Contains(html, `<table class="write-word-preview__table">`) {
		t.Fatalf("missing table html: %s", html)
	}
	if !strings.Contains(html, ">中文<") || !strings.Contains(html, "write-word-preview__latin") || !strings.Contains(html, "English") {
		t.Fatalf("missing table cell content or latin span: %s", html)
	}
}

func TestDocxPreviewImage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "image.docx")
	png := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00}
	documentXML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"
  xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"
  xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  <w:body>
    <w:p>
      <w:r>
        <w:drawing>
          <a:graphic>
            <a:graphicData>
              <pic:pic xmlns:pic="http://schemas.openxmlformats.org/drawingml/2006/picture">
                <pic:blipFill>
                  <a:blip r:embed="rIdImage"/>
                </pic:blipFill>
              </pic:pic>
            </a:graphicData>
          </a:graphic>
        </w:drawing>
      </w:r>
    </w:p>
  </w:body>
</w:document>`
	rels := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rIdImage" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="media/image1.png"/>
</Relationships>`
	if err := writeRawDocxWithMedia(path, documentXML, rels, "word/media/image1.png", png); err != nil {
		t.Fatal(err)
	}
	html, err := readDocxHTML(path)
	if err != nil {
		t.Fatalf("html: %v", err)
	}
	if !strings.Contains(html, `<img class="write-word-preview__image"`) {
		t.Fatalf("missing image html: %s", html)
	}
}

func writeRawDocxWithMedia(path, documentXML, relsXML, mediaPath string, media []byte) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	zipWriter := zip.NewWriter(file)
	entries := []struct {
		name string
		data string
	}{
		{"[Content_Types].xml", docxContentTypes},
		{"_rels/.rels", docxRootRels},
		{"word/_rels/document.xml.rels", relsXML},
		{"word/document.xml", documentXML},
	}
	for _, entry := range entries {
		writer, err := zipWriter.Create(entry.name)
		if err != nil {
			return err
		}
		if _, err := io.Copy(writer, bytes.NewBufferString(entry.data)); err != nil {
			return err
		}
	}
	writer, err := zipWriter.Create(mediaPath)
	if err != nil {
		return err
	}
	if _, err := writer.Write(media); err != nil {
		return err
	}
	return zipWriter.Close()
}

func writeRawDocx(path, documentXML string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	zipWriter := zip.NewWriter(file)
	entries := []struct {
		name string
		data string
	}{
		{"[Content_Types].xml", docxContentTypes},
		{"_rels/.rels", docxRootRels},
		{"word/_rels/document.xml.rels", docxDocumentRels},
		{"word/document.xml", documentXML},
	}
	for _, entry := range entries {
		writer, err := zipWriter.Create(entry.name)
		if err != nil {
			return err
		}
		if _, err := io.Copy(writer, bytes.NewBufferString(entry.data)); err != nil {
			return err
		}
	}
	return zipWriter.Close()
}
