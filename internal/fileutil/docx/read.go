// Package docx reads plain text from Office Open XML (.docx) files.
package docx

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

// ReadPlainText extracts paragraph text from a .docx file.
func ReadPlainText(path string) (string, error) {
	rc, err := openDocumentXML(path)
	if err != nil {
		return "", err
	}
	defer rc.Close()
	doc, err := parseDocument(rc)
	if err != nil {
		return "", err
	}
	return doc.plainText(), nil
}

func openDocumentXML(path string) (io.ReadCloser, error) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	var docXML *zip.File
	for _, file := range reader.File {
		if file.Name == "word/document.xml" {
			docXML = file
			break
		}
	}
	if docXML == nil {
		reader.Close()
		return nil, fmt.Errorf("invalid docx: missing word/document.xml")
	}
	rc, err := docXML.Open()
	if err != nil {
		reader.Close()
		return nil, err
	}
	return &zipReadCloser{ReadCloser: rc, closer: reader.Close}, nil
}

type zipReadCloser struct {
	io.ReadCloser
	closer func() error
}

func (z *zipReadCloser) Close() error {
	err := z.ReadCloser.Close()
	if closeErr := z.closer(); closeErr != nil && err == nil {
		return closeErr
	}
	return err
}

type paragraph struct {
	style string
	runs  []run
}

type run struct {
	text      string
	bold      bool
	italic    bool
	lineBreak bool
}

type document struct {
	paragraphs []paragraph
}

func (d document) plainText() string {
	if len(d.paragraphs) == 0 {
		return ""
	}
	lines := make([]string, 0, len(d.paragraphs))
	for _, para := range d.paragraphs {
		var parts []string
		for _, r := range para.runs {
			if r.lineBreak {
				parts = append(parts, "\n")
				continue
			}
			parts = append(parts, r.text)
		}
		lines = append(lines, strings.Join(parts, ""))
	}
	return strings.Join(lines, "\n")
}

func parseDocument(r io.Reader) (document, error) {
	decoder := xml.NewDecoder(r)
	var doc document
	var currentPara *paragraph
	var currentRun *run
	var inRun bool
	var runBold bool
	var runItalic bool
	var fields FieldFilter

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
		currentRun.bold = runBold
		currentRun.italic = runItalic
		currentPara.runs = append(currentPara.runs, *currentRun)
		resetRun()
	}

	flushPara := func() {
		flushRun()
		if currentPara == nil {
			return
		}
		doc.paragraphs = append(doc.paragraphs, *currentPara)
		currentPara = nil
	}

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return document{}, err
		}
		switch node := token.(type) {
		case xml.StartElement:
			fields.OnStart(node.Name.Local, node.Attr)
			switch node.Name.Local {
			case "p":
				flushPara()
				currentPara = &paragraph{}
			case "pStyle":
				if currentPara != nil {
					currentPara.style = xmlAttr(node.Attr, "val")
				}
			case "r":
				flushRun()
				inRun = true
				currentRun = &run{}
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
					currentPara.runs = append(currentPara.runs, run{lineBreak: true})
				}
			case "tab":
				if inRun && currentRun != nil {
					currentRun.text += "\t"
				}
			case "t":
				if inRun && currentRun != nil {
					var text string
					if err := decoder.DecodeElement(&text, &node); err != nil {
						return document{}, err
					}
					if fields.AllowText() {
						currentRun.text += text
					}
				}
			}
		case xml.EndElement:
			fields.OnEnd(node.Name.Local)
			switch node.Name.Local {
			case "p":
				flushPara()
			case "r":
				flushRun()
			}
		case xml.CharData:
			if inRun && currentRun != nil && fields.AllowText() {
				currentRun.text += string(node)
			}
		}
	}
	flushPara()
	return doc, nil
}

func xmlAttr(attrs []xml.Attr, local string) string {
	for _, attr := range attrs {
		if attr.Name.Local == local {
			return attr.Value
		}
	}
	return ""
}

// IsDocx reports whether path has a .docx extension.
func IsDocx(path string) bool {
	return strings.EqualFold(filepath.Ext(strings.TrimSpace(path)), ".docx")
}
