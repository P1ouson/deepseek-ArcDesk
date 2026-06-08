package wecom

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// Crypt implements WeCom callback message encryption (WXBizMsgCrypt).
type Crypt struct {
	token  string
	aesKey []byte
	corpID string
}

// NewCrypt builds a Crypt from callback token, EncodingAESKey, and corp id.
func NewCrypt(token, encodingAESKey, corpID string) (*Crypt, error) {
	key, err := decodeAESKey(encodingAESKey)
	if err != nil {
		return nil, err
	}
	return &Crypt{
		token:  strings.TrimSpace(token),
		aesKey: key,
		corpID: strings.TrimSpace(corpID),
	}, nil
}

func decodeAESKey(raw string) ([]byte, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil, errors.New("encoding aes key is required")
	}
	if len(s) != 43 {
		return nil, fmt.Errorf("encoding aes key must be 43 characters, got %d", len(s))
	}
	key, err := base64.StdEncoding.DecodeString(s + "=")
	if err != nil {
		return nil, fmt.Errorf("decode encoding aes key: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("encoding aes key must decode to 32 bytes, got %d", len(key))
	}
	return key, nil
}

// VerifyURL decrypts echostr for the GET URL verification handshake.
func (c *Crypt) VerifyURL(msgSignature, timestamp, nonce, echostr string) (string, error) {
	sig := c.sign(timestamp, nonce, echostr)
	if !strings.EqualFold(sig, strings.TrimSpace(msgSignature)) {
		return "", errors.New("invalid msg_signature")
	}
	plain, err := c.decrypt(echostr)
	if err != nil {
		return "", err
	}
	return plain, nil
}

// DecryptMsg decrypts an encrypted callback payload after signature check.
func (c *Crypt) DecryptMsg(msgSignature, timestamp, nonce, encrypted string) (string, error) {
	sig := c.sign(timestamp, nonce, encrypted)
	if !strings.EqualFold(sig, strings.TrimSpace(msgSignature)) {
		return "", errors.New("invalid msg_signature")
	}
	return c.decrypt(encrypted)
}

func (c *Crypt) sign(timestamp, nonce, encrypted string) string {
	parts := []string{c.token, strings.TrimSpace(timestamp), strings.TrimSpace(nonce), strings.TrimSpace(encrypted)}
	sort.Strings(parts)
	h := sha1.New()
	h.Write([]byte(strings.Join(parts, "")))
	return hex.EncodeToString(h.Sum(nil))
}

func (c *Crypt) decrypt(encrypted string) (string, error) {
	cipherData, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encrypted))
	if err != nil {
		return "", fmt.Errorf("decode encrypted payload: %w", err)
	}
	plain, err := c.aesDecrypt(cipherData)
	if err != nil {
		return "", err
	}
	plain, err = pkcs7Unpad(plain, aes.BlockSize)
	if err != nil {
		return "", err
	}
	if len(plain) < 20 {
		return "", errors.New("decrypted payload too short")
	}
	msgLen := binary.BigEndian.Uint32(plain[16:20])
	if int(msgLen)+20 > len(plain) {
		return "", errors.New("invalid decrypted message length")
	}
	msg := string(plain[20 : 20+msgLen])
	receiveID := string(plain[20+msgLen:])
	if c.corpID != "" && receiveID != c.corpID {
		return "", errors.New("receive id mismatch")
	}
	return msg, nil
}

func (c *Crypt) encrypt(plain string) (string, error) {
	randBytes := make([]byte, 16)
	if _, err := rand.Read(randBytes); err != nil {
		return "", err
	}
	msgBytes := []byte(plain)
	buf := make([]byte, 16+4+len(msgBytes)+len(c.corpID))
	copy(buf[:16], randBytes)
	binary.BigEndian.PutUint32(buf[16:20], uint32(len(msgBytes)))
	copy(buf[20:], msgBytes)
	copy(buf[20+len(msgBytes):], c.corpID)
	padded, err := pkcs7Pad(buf, aes.BlockSize)
	if err != nil {
		return "", err
	}
	out, err := c.aesEncrypt(padded)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(out), nil
}

func (c *Crypt) aesEncrypt(plain []byte) ([]byte, error) {
	block, err := aes.NewCipher(c.aesKey)
	if err != nil {
		return nil, err
	}
	iv := c.aesKey[:aes.BlockSize]
	mode := cipher.NewCBCEncrypter(block, iv)
	out := make([]byte, len(plain))
	mode.CryptBlocks(out, plain)
	return out, nil
}

func (c *Crypt) aesDecrypt(cipherData []byte) ([]byte, error) {
	if len(cipherData)%aes.BlockSize != 0 {
		return nil, errors.New("invalid cipher length")
	}
	block, err := aes.NewCipher(c.aesKey)
	if err != nil {
		return nil, err
	}
	iv := c.aesKey[:aes.BlockSize]
	mode := cipher.NewCBCDecrypter(block, iv)
	out := make([]byte, len(cipherData))
	mode.CryptBlocks(out, cipherData)
	return out, nil
}

func pkcs7Pad(data []byte, blockSize int) ([]byte, error) {
	if blockSize <= 0 || blockSize >= 256 {
		return nil, errors.New("invalid block size")
	}
	padding := blockSize - len(data)%blockSize
	if padding == 0 {
		padding = blockSize
	}
	return append(data, bytes.Repeat([]byte{byte(padding)}, padding)...), nil
}

func pkcs7Unpad(data []byte, blockSize int) ([]byte, error) {
	if len(data) == 0 || len(data)%blockSize != 0 {
		return nil, errors.New("invalid padded data")
	}
	padding := int(data[len(data)-1])
	if padding <= 0 || padding > blockSize || padding > len(data) {
		return nil, errors.New("invalid pkcs7 padding")
	}
	for _, b := range data[len(data)-padding:] {
		if int(b) != padding {
			return nil, errors.New("invalid pkcs7 padding bytes")
		}
	}
	return data[:len(data)-padding], nil
}
