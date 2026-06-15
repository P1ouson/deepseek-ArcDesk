package main

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"unicode"

	"arcdesk/internal/fileutil/docx"
)

func writeDocumentExt(path string) string {
	return strings.ToLower(filepath.Ext(strings.TrimSpace(path)))
}

func isDocxDocument(path string) bool {
	return writeDocumentExt(path) == ".docx"
}

func isLegacyDocDocument(path string) bool {
	return writeDocumentExt(path) == ".doc"
}

func readWriteDocument(path string) (string, error) {
	switch writeDocumentExt(path) {
	case ".docx":
		return readDocxPlainText(path)
	case ".doc":
		return readLegacyDoc(path)
	default:
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
}

func readWriteDocumentPreview(path string) (string, error) {
	switch writeDocumentExt(path) {
	case ".docx":
		return readDocxHTML(path)
	case ".doc":
		body, err := readLegacyDoc(path)
		if err != nil {
			return "", err
		}
		return plainTextPreviewHTML(body), nil
	default:
		return "", nil
	}
}

func writeWriteDocument(path string, content string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if isDocxDocument(path) {
		return writeDocxPlainText(path, content)
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func readDocxPlainText(path string) (string, error) {
	text, err := docx.ReadPlainText(path)
	if err == nil {
		return text, nil
	}
	if isInvalidDocxZip(err) {
		data, readErr := os.ReadFile(path)
		if readErr == nil {
			return string(data), nil
		}
	}
	return "", err
}

func readDocxHTML(path string) (string, error) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		if isInvalidDocxZip(err) {
			body, plainErr := readDocxPlainText(path)
			if plainErr == nil {
				return plainTextPreviewHTML(body), nil
			}
		}
		return "", err
	}
	defer reader.Close()

	assets, err := loadDocxPreviewAssets(reader)
	if err != nil {
		return "", err
	}
	docXML, err := readZipBytes(reader, "word/document.xml")
	if err != nil {
		return "", err
	}
	doc, err := parseDocxPreviewBody(bytes.NewReader(docXML), assets)
	if err != nil {
		return "", err
	}
	return doc.html(), nil
}

func isInvalidDocxZip(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "not a valid zip") || strings.Contains(msg, "invalid docx")
}

type docxPreviewAssets struct {
	images map[string]docxPreviewImage
}

type docxPreviewImage struct {
	mime string
	data []byte
}

func loadDocxPreviewAssets(reader *zip.ReadCloser) (docxPreviewAssets, error) {
	assets := docxPreviewAssets{images: map[string]docxPreviewImage{}}
	relsXML, err := readZipBytes(reader, "word/_rels/document.xml.rels")
	if err != nil {
		return assets, nil
	}
	targets := parseDocxRelationships(relsXML)
	for id, target := range targets {
		entry := resolveDocxMediaPath(target)
		data, err := readZipBytes(reader, entry)
		if err != nil {
			base := filepath.Base(entry)
			for _, file := range reader.File {
				if filepath.Base(file.Name) != base {
					continue
				}
				data, err = readZipFileEntry(file)
				if err == nil {
					entry = file.Name
					break
				}
			}
		}
		if err != nil || len(data) == 0 {
			continue
		}
		assets.images[id] = docxPreviewImage{mime: mimeFromDocxPath(entry), data: data}
	}
	return assets, nil
}

func resolveDocxMediaPath(target string) string {
	target = strings.ReplaceAll(strings.TrimSpace(target), "\\", "/")
	if strings.HasPrefix(target, "/") {
		return strings.TrimPrefix(target, "/")
	}
	if strings.HasPrefix(target, "word/") {
		return target
	}
	return "word/" + strings.TrimPrefix(target, "./")
}

func parseDocxRelationships(data []byte) map[string]string {
	out := map[string]string{}
	decoder := xml.NewDecoder(bytes.NewReader(data))
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return out
		}
		node, ok := token.(xml.StartElement)
		if !ok || node.Name.Local != "Relationship" {
			continue
		}
		id := xmlAttr(node.Attr, "Id")
		target := xmlAttr(node.Attr, "Target")
		if id != "" && target != "" {
			out[id] = target
		}
	}
	return out
}

func readZipBytes(reader *zip.ReadCloser, name string) ([]byte, error) {
	for _, file := range reader.File {
		if file.Name != name {
			continue
		}
		return readZipFileEntry(file)
	}
	return nil, fmt.Errorf("zip entry not found: %s", name)
}

func readZipFileEntry(file *zip.File) ([]byte, error) {
	rc, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

func mimeFromDocxPath(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".bmp":
		return "image/bmp"
	case ".svg":
		return "image/svg+xml"
	default:
		return "application/octet-stream"
	}
}

type docxPreviewRun struct {
	text     string
	bold     bool
	italic   bool
	lineBreak bool
	imageURI string
}

type docxPreviewParagraph struct {
	style string
	runs  []docxPreviewRun
}

type docxPreviewCell struct {
	paragraphs []docxPreviewParagraph
}

type docxPreviewRow struct {
	cells []docxPreviewCell
}

type docxPreviewTable struct {
	rows []docxPreviewRow
}

type docxPreviewBlock struct {
	para  *docxPreviewParagraph
	table *docxPreviewTable
}

type docxPreviewDocument struct {
	blocks []docxPreviewBlock
}

func (d docxPreviewDocument) html() string {
	if len(d.blocks) == 0 {
		return `<div class="write-word-preview"><p></p></div>`
	}
	var parts []string
	for _, block := range d.blocks {
		switch {
		case block.table != nil:
			parts = append(parts, renderPreviewTableHTML(block.table))
		case block.para != nil:
			parts = append(parts, renderPreviewParagraphHTML(block.para))
		}
	}
	return `<div class="write-word-preview">` + strings.Join(parts, "") + `</div>`
}

func renderPreviewTableHTML(table *docxPreviewTable) string {
	if table == nil || len(table.rows) == 0 {
		return ""
	}
	var rows []string
	for _, row := range table.rows {
		var cells []string
		for _, cell := range row.cells {
			var inner strings.Builder
			for i, para := range cell.paragraphs {
				if i > 0 {
					inner.WriteString("<br/>")
				}
				inner.WriteString(renderPreviewParagraphInnerHTML(&para))
			}
			cells = append(cells, `<td class="write-word-preview__td">`+inner.String()+`</td>`)
		}
		rows = append(rows, `<tr>`+strings.Join(cells, "")+`</tr>`)
	}
	return `<table class="write-word-preview__table"><tbody>` + strings.Join(rows, "") + `</tbody></table>`
}

func renderPreviewParagraphHTML(para *docxPreviewParagraph) string {
	if para == nil {
		return ""
	}
	tag := paragraphTag(para.style)
	inner := renderPreviewParagraphInnerHTML(para)
	if inner == "" {
		return fmt.Sprintf("<%s></%s>", tag, tag)
	}
	return fmt.Sprintf("<%s>%s</%s>", tag, inner, tag)
}

func renderPreviewParagraphInnerHTML(para *docxPreviewParagraph) string {
	var inner strings.Builder
	for _, run := range para.runs {
		if run.lineBreak {
			inner.WriteString("<br/>")
			continue
		}
		if run.imageURI != "" {
			inner.WriteString(`<img class="write-word-preview__image" src="`)
			inner.WriteString(run.imageURI)
			inner.WriteString(`" alt="" />`)
			continue
		}
		text := renderMixedScriptHTML(run.text)
		if text == "" {
			continue
		}
		switch {
		case run.bold && run.italic:
			inner.WriteString("<strong><em>")
			inner.WriteString(text)
			inner.WriteString("</em></strong>")
		case run.bold:
			inner.WriteString("<strong>")
			inner.WriteString(text)
			inner.WriteString("</strong>")
		case run.italic:
			inner.WriteString("<em>")
			inner.WriteString(text)
			inner.WriteString("</em>")
		default:
			inner.WriteString(text)
		}
	}
	return inner.String()
}

func renderMixedScriptHTML(text string) string {
	if text == "" {
		return ""
	}
	type segment struct {
		latin bool
		text  string
	}
	var segments []segment
	flush := func(latin bool, buf *strings.Builder) {
		if buf.Len() == 0 {
			return
		}
		segments = append(segments, segment{latin: latin, text: buf.String()})
		buf.Reset()
	}
	var buf strings.Builder
	currentLatin := false
	started := false
	for _, r := range text {
		latin := isLatinPreviewRune(r)
		if !started {
			currentLatin = latin
			started = true
		}
		if latin != currentLatin {
			flush(currentLatin, &buf)
			currentLatin = latin
		}
		buf.WriteRune(r)
	}
	flush(currentLatin, &buf)

	var out strings.Builder
	for _, seg := range segments {
		escaped := html.EscapeString(seg.text)
		if seg.latin {
			out.WriteString(`<span class="write-word-preview__latin">`)
			out.WriteString(escaped)
			out.WriteString(`</span>`)
		} else {
			out.WriteString(escaped)
		}
	}
	return out.String()
}

func isLatinPreviewRune(r rune) bool {
	if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
		return true
	}
	return unicode.In(r, unicode.Latin, unicode.Greek)
}

func parseDocxPreviewBody(r io.Reader, assets docxPreviewAssets) (docxPreviewDocument, error) {
	decoder := xml.NewDecoder(r)
	var doc docxPreviewDocument
	var table *docxPreviewTable
	var row *docxPreviewRow
	var cell *docxPreviewCell
	var currentPara *docxPreviewParagraph
	var currentRun *docxPreviewRun
	var inRun bool
	var runBold bool
	var runItalic bool
	var fields docx.FieldFilter

	resetRun := func() {
		currentRun = nil
		runBold = false
		runItalic = false
		inRun = false
	}

	flushRun := func() {
		if currentPara == nil || currentRun == nil {
			resetRun()
			return
		}
		if currentRun.text == "" && currentRun.imageURI == "" && !currentRun.lineBreak {
			resetRun()
			return
		}
		currentRun.bold = runBold
		currentRun.italic = runItalic
		currentPara.runs = append(currentPara.runs, *currentRun)
		resetRun()
	}

	flushParagraph := func() {
		flushRun()
		if currentPara == nil {
			return
		}
		if len(currentPara.runs) == 0 {
			currentPara = nil
			return
		}
		if cell != nil {
			cell.paragraphs = append(cell.paragraphs, *currentPara)
		} else if table == nil {
			para := currentPara
			doc.blocks = append(doc.blocks, docxPreviewBlock{para: para})
		}
		currentPara = nil
	}

	flushTable := func() {
		flushParagraph()
		if table == nil || len(table.rows) == 0 {
			table = nil
			return
		}
		doc.blocks = append(doc.blocks, docxPreviewBlock{table: table})
		table = nil
		row = nil
		cell = nil
	}

	startParagraph := func() {
		flushParagraph()
		currentPara = &docxPreviewParagraph{}
	}

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return docxPreviewDocument{}, err
		}
		switch node := token.(type) {
		case xml.StartElement:
			fields.OnStart(node.Name.Local, node.Attr)
			switch node.Name.Local {
			case "tbl":
				flushParagraph()
				table = &docxPreviewTable{}
				row = nil
				cell = nil
			case "tr":
				if table != nil {
					row = &docxPreviewRow{}
					table.rows = append(table.rows, *row)
					row = &table.rows[len(table.rows)-1]
				}
			case "tc":
				if row != nil {
					cell = &docxPreviewCell{}
					row.cells = append(row.cells, *cell)
					cell = &row.cells[len(row.cells)-1]
				}
			case "p":
				startParagraph()
			case "pStyle":
				if currentPara != nil {
					currentPara.style = xmlAttr(node.Attr, "val")
				}
			case "r":
				flushRun()
				inRun = true
				currentRun = &docxPreviewRun{}
			case "b", "i":
				if inRun {
					if node.Name.Local == "b" {
						runBold = true
					} else {
						runItalic = true
					}
				}
			case "br":
				if currentPara != nil {
					flushRun()
					currentPara.runs = append(currentPara.runs, docxPreviewRun{lineBreak: true})
				}
			case "tab":
				if inRun && currentRun != nil && fields.AllowText() {
					currentRun.text += "\t"
				}
			case "blip", "imagedata":
				if currentPara == nil {
					startParagraph()
				}
				flushRun()
				inRun = true
				currentRun = &docxPreviewRun{}
				if id := docxRelID(node.Attr); id != "" {
					if img, ok := assets.images[id]; ok && len(img.data) > 0 {
						currentRun.imageURI = imageDataURI(img.mime, img.data)
					}
				}
				flushRun()
			case "t":
				if inRun && currentRun != nil {
					var text string
					if err := decoder.DecodeElement(&text, &node); err != nil {
						return docxPreviewDocument{}, err
					}
					if fields.AllowText() {
						currentRun.text += text
					}
				}
			}
		case xml.EndElement:
			fields.OnEnd(node.Name.Local)
			switch node.Name.Local {
			case "tbl":
				flushTable()
			case "tc":
				cell = nil
			case "tr":
				row = nil
			case "p":
				flushParagraph()
			case "r":
				flushRun()
			}
		case xml.CharData:
			if inRun && currentRun != nil && fields.AllowText() {
				currentRun.text += string(node)
			}
		}
	}
	flushTable()
	flushParagraph()
	return doc, nil
}

func docxRelID(attrs []xml.Attr) string {
	for _, attr := range attrs {
		switch attr.Name.Local {
		case "embed", "id", "link":
			if attr.Value != "" {
				return attr.Value
			}
		}
	}
	return ""
}

func imageDataURI(mime string, data []byte) string {
	if len(data) == 0 {
		return ""
	}
	return fmt.Sprintf("data:%s;base64,%s", mime, base64.StdEncoding.EncodeToString(data))
}

func paragraphTag(style string) string {
	switch strings.ToLower(strings.TrimSpace(style)) {
	case "title", "heading1", "heading 1", "1":
		return "h1"
	case "subtitle", "heading2", "heading 2", "2":
		return "h2"
	case "heading3", "heading 3", "3":
		return "h3"
	case "heading4", "heading 4", "4":
		return "h4"
	case "heading5", "heading 5", "5":
		return "h5"
	case "heading6", "heading 6", "6":
		return "h6"
	default:
		return "p"
	}
}

func xmlAttr(attrs []xml.Attr, local string) string {
	for _, attr := range attrs {
		if attr.Name.Local == local {
			return attr.Value
		}
	}
	return ""
}

func plainTextPreviewHTML(text string) string {
	return `<div class="write-word-preview"><pre class="write-word-preview__plain">` + html.EscapeString(text) + `</pre></div>`
}

func readLegacyDoc(path string) (string, error) {
	if runtime.GOOS == "windows" {
		if text, err := readLegacyDocWindows(path); err == nil && strings.TrimSpace(text) != "" {
			return text, nil
		}
	}
	return "", fmt.Errorf("legacy .doc is not supported on this system; save as .docx in Word and open again")
}

func readLegacyDocWindows(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	abs = strings.ReplaceAll(abs, "'", "''")
	script := fmt.Sprintf(`
$ErrorActionPreference = 'Stop'
$word = New-Object -ComObject Word.Application
$word.Visible = $false
try {
  $doc = $word.Documents.Open('%s')
  $text = $doc.Content.Text
  $doc.Close($false)
  [Console]::OutputEncoding = [Text.UTF8Encoding]::UTF8
  Write-Output $text
} finally {
  $word.Quit()
}
`, abs)
	out, err := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return strings.ReplaceAll(string(out), "\r", ""), nil
}

const docxContentTypes = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
</Types>`

const docxRootRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`

const docxDocumentRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"></Relationships>`

func writeDocxPlainText(path string, content string) error {
	body := buildDocxBodyXML(content)
	documentXML := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>%s</w:body>
</w:document>`, body)

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

func buildDocxBodyXML(content string) string {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		lines = []string{""}
	}
	var parts []string
	for _, line := range lines {
		escaped := escapeDocxText(line)
		if escaped == "" {
			parts = append(parts, "<w:p/>")
			continue
		}
		parts = append(parts, fmt.Sprintf("<w:p><w:r><w:t xml:space=\"preserve\">%s</w:t></w:r></w:p>", escaped))
	}
	return strings.Join(parts, "")
}

func escapeDocxText(text string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		"\"", "&quot;",
		"'", "&apos;",
	)
	return replacer.Replace(text)
}
