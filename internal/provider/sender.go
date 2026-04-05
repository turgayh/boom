package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type SendRequest struct {
	To      string `json:"to"`
	Channel string `json:"channel"`
	Content string `json:"content"`
}

type SendResponse struct {
	MessageID string `json:"messageId"`
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
}

type Sender interface {
	Send(ctx context.Context, req *SendRequest) (*SendResponse, error)
}

type webhookSender struct {
	webhookURL string
	client     *http.Client
}

func NewSender(webhookURL string) Sender {
	return &webhookSender{
		webhookURL: webhookURL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (s *webhookSender) Send(ctx context.Context, req *SendRequest) (*SendResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, s.webhookURL, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var sendResp SendResponse
	if err := json.NewDecoder(resp.Body).Decode(&sendResp); err != nil {
		sendResp = SendResponse{
			MessageID: uuid.New().String(),
			Status:    "accepted",
		}
	}

	return &sendResp, nil
}
