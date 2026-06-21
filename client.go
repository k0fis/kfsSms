package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type SmsClient struct {
	baseUrl    string
	authHeader string
	client     *http.Client
}

type OutgoingSms struct {
	ID   string `json:"id"`
	Numb string `json:"numb"`
	Text string `json:"text"`
}

func NewSmsClient(baseUrl, user, password string) *SmsClient {
	cred := base64.StdEncoding.EncodeToString([]byte(user + ":" + password))
	return &SmsClient{
		baseUrl:    strings.TrimRight(baseUrl, "/"),
		authHeader: "Basic " + cred,
		client:     &http.Client{Timeout: 15 * time.Second},
	}
}

// PollOutgoing fetches one pending outgoing SMS from the server.
func (c *SmsClient) PollOutgoing() (*OutgoingSms, error) {
	req, err := http.NewRequest("GET", c.baseUrl+"/sms/o", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", c.authHeader)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("poll request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("poll: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	s := strings.TrimSpace(string(body))
	if s == "" || s == "{}" {
		return nil, nil
	}

	var sms OutgoingSms
	if err := json.Unmarshal(body, &sms); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	if sms.ID == "" {
		return nil, nil
	}

	return &sms, nil
}

// ReportIncoming sends an incoming SMS to the server.
func (c *SmsClient) ReportIncoming(number, text string, t time.Time) error {
	payload := fmt.Sprintf(`{"numb":"%s","text":"%s","time":"%s"}`,
		escapeJSON(number), escapeJSON(text), t.Format(time.RFC3339))
	return c.post("/sms/i", payload)
}

// ConfirmSent confirms that an SMS was successfully sent.
func (c *SmsClient) ConfirmSent(id string) error {
	payload := fmt.Sprintf(`{"id":"%s","time":"%s"}`,
		escapeJSON(id), time.Now().Format(time.RFC3339))
	return c.post("/sms/c", payload)
}

// ReportFail reports that sending an SMS failed.
func (c *SmsClient) ReportFail(id, msg string) error {
	payload := fmt.Sprintf(`{"id":"%s","time":"%s","mesg":"%s"}`,
		escapeJSON(id), time.Now().Format(time.RFC3339), escapeJSON(msg))
	return c.post("/sms/f", payload)
}

func (c *SmsClient) post(path, data string) error {
	encoded := "data=" + url.QueryEscape(data)

	req, err := http.NewRequest("POST", c.baseUrl+path, strings.NewReader(encoded))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", c.authHeader)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("POST %s failed: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		slog.Warn("POST failed", "path", path, "status", resp.StatusCode, "body", string(body))
		return fmt.Errorf("POST %s: HTTP %d", path, resp.StatusCode)
	}

	slog.Debug("POST ok", "path", path, "status", resp.StatusCode)
	return nil
}

func escapeJSON(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", `\r`)
	s = strings.ReplaceAll(s, "\t", `\t`)
	return s
}
