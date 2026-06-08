package wecom

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const wecomAPIBase = "https://qyapi.weixin.qq.com/cgi-bin"

// Client calls WeCom APIs for a single application.
type Client struct {
	corpID  string
	secret  string
	agentID int64
	http    *http.Client

	mu          sync.Mutex
	token       string
	tokenExpire time.Time
}

// NewClient builds a WeCom API client.
func NewClient(corpID, secret, agentID string) (*Client, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(agentID), 10, 64)
	if err != nil || id <= 0 {
		return nil, fmt.Errorf("invalid agent id: %q", agentID)
	}
	corpID = strings.TrimSpace(corpID)
	secret = strings.TrimSpace(secret)
	if corpID == "" || secret == "" {
		return nil, fmt.Errorf("corp id and secret are required")
	}
	return &Client{
		corpID:  corpID,
		secret:  secret,
		agentID: id,
		http:    &http.Client{Timeout: 15 * time.Second},
	}, nil
}

// Ping verifies credentials by fetching an access token.
func (c *Client) Ping() error {
	_, err := c.accessToken()
	return err
}

// SendText sends an application text message to one user id.
func (c *Client) SendText(toUser, content string) error {
	toUser = strings.TrimSpace(toUser)
	content = strings.TrimSpace(content)
	if toUser == "" {
		return fmt.Errorf("to user is required")
	}
	if content == "" {
		return fmt.Errorf("message text is required")
	}
	token, err := c.accessToken()
	if err != nil {
		return err
	}
	payload := map[string]any{
		"touser":  toUser,
		"msgtype": "text",
		"agentid": c.agentID,
		"text": map[string]string{
			"content": content,
		},
	}
	body, _ := json.Marshal(payload)
	endpoint := wecomAPIBase + "/message/send?access_token=" + url.QueryEscape(token)
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var out struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return fmt.Errorf("decode send response: %w", err)
	}
	if out.ErrCode != 0 {
		return fmt.Errorf("wecom send failed: %s (%d)", out.ErrMsg, out.ErrCode)
	}
	return nil
}

func (c *Client) accessToken() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token != "" && time.Now().Before(c.tokenExpire) {
		return c.token, nil
	}
	endpoint := wecomAPIBase + "/gettoken?corpid=" + url.QueryEscape(c.corpID) + "&corpsecret=" + url.QueryEscape(c.csecret())
	resp, err := c.http.Get(endpoint)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var out struct {
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("decode token response: %w", err)
	}
	if out.ErrCode != 0 {
		return "", fmt.Errorf("wecom token failed: %s (%d)", out.ErrMsg, out.ErrCode)
	}
	if strings.TrimSpace(out.AccessToken) == "" {
		return "", fmt.Errorf("wecom token missing in response")
	}
	c.token = out.AccessToken
	ttl := out.ExpiresIn
	if ttl <= 0 {
		ttl = 7200
	}
	c.tokenExpire = time.Now().Add(time.Duration(ttl-120) * time.Second)
	return c.token, nil
}

func (c *Client) csecret() string {
	return c.secret
}
