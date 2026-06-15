package docx

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadPlainText(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.docx")
	documentXML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p><w:r><w:t>Line one</w:t></w:r></w:p>
    <w:p><w:r><w:t>Line two</w:t></w:r></w:p>
  </w:body>
</w:document>`
	if err := writeRawDocx(path, documentXML); err != nil {
		t.Fatal(err)
	}
	got, err := ReadPlainText(path)
	if err != nil {
		t.Fatalf("ReadPlainText: %v", err)
	}
	want := "Line one\nLine two"
	if strings.TrimSpace(got) != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func writeRawDocx(path, documentXML string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	zipWriter := zip.NewWriter(file)
	entries := map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
</Types>`,
		"_rels/.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/_rels/document.xml.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"></Relationships>`,
		"word/document.xml":            documentXML,
	}
	for name, data := range entries {
		writer, err := zipWriter.Create(name)
		if err != nil {
			return err
		}
		if _, err := io.Copy(writer, bytes.NewBufferString(data)); err != nil {
			return err
		}
	}
	return zipWriter.Close()
}
