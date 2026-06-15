package docx

import "encoding/xml"

// FieldFilter skips Word field instructions (TOC, HYPERLINK, PAGEREF, etc.).
type FieldFilter struct {
	inInstrText  bool
	fieldDepth   int
	fieldCollect bool
}

func (f *FieldFilter) OnStart(name string, attrs []xml.Attr) {
	switch name {
	case "instrText":
		f.inInstrText = true
	case "fldChar":
		switch xmlAttr(attrs, "fldCharType") {
		case "begin":
			f.fieldDepth++
			if f.fieldDepth == 1 {
				f.fieldCollect = false
			}
		case "separate":
			if f.fieldDepth == 1 {
				f.fieldCollect = true
			}
		case "end":
			if f.fieldDepth == 1 {
				f.fieldCollect = false
			}
			if f.fieldDepth > 0 {
				f.fieldDepth--
			}
		}
	case "fldSimple":
		f.fieldDepth++
		if f.fieldDepth == 1 {
			f.fieldCollect = false
		}
	}
}

func (f *FieldFilter) OnEnd(name string) {
	switch name {
	case "instrText":
		f.inInstrText = false
	case "fldSimple":
		if f.fieldDepth == 1 {
			f.fieldCollect = false
		}
		if f.fieldDepth > 0 {
			f.fieldDepth--
		}
	}
}

func (f *FieldFilter) AllowText() bool {
	if f.inInstrText {
		return false
	}
	if f.fieldDepth == 0 {
		return true
	}
	return f.fieldCollect
}
