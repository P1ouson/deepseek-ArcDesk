package wecom

import (
	"encoding/xml"
	"strings"
)

// IncomingMessage is a decrypted WeCom callback message.
type IncomingMessage struct {
	ToUserName   string
	FromUserName string
	CreateTime   int64
	MsgType      string
	Content      string
	MsgID        int64
	AgentID      int64
	Event        string
}

type encryptedEnvelope struct {
	XMLName xml.Name `xml:"xml"`
	Encrypt string   `xml:"Encrypt"`
}

type incomingXML struct {
	XMLName      xml.Name `xml:"xml"`
	ToUserName   string   `xml:"ToUserName"`
	FromUserName string   `xml:"FromUserName"`
	CreateTime   int64    `xml:"CreateTime"`
	MsgType      string   `xml:"MsgType"`
	Content      string   `xml:"Content"`
	MsgID        int64    `xml:"MsgId"`
	AgentID      int64    `xml:"AgentID"`
	Event        string   `xml:"Event"`
}

// ParseEncryptedEnvelope extracts Encrypt from the callback POST body.
func ParseEncryptedEnvelope(body []byte) (string, error) {
	var env encryptedEnvelope
	if err := xml.Unmarshal(body, &env); err != nil {
		return "", err
	}
	return strings.TrimSpace(env.Encrypt), nil
}

// ParseIncomingMessage parses decrypted callback XML.
func ParseIncomingMessage(body []byte) (IncomingMessage, error) {
	var raw incomingXML
	if err := xml.Unmarshal(body, &raw); err != nil {
		return IncomingMessage{}, err
	}
	return IncomingMessage{
		ToUserName:   strings.TrimSpace(raw.ToUserName),
		FromUserName: strings.TrimSpace(raw.FromUserName),
		CreateTime:   raw.CreateTime,
		MsgType:      strings.TrimSpace(raw.MsgType),
		Content:      strings.TrimSpace(raw.Content),
		MsgID:        raw.MsgID,
		AgentID:      raw.AgentID,
		Event:        strings.TrimSpace(raw.Event),
	}, nil
}
