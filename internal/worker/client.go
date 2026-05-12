package worker

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"adp/internal/model"
)

type Client struct {
	serverURL    string
	workerToken  string
	name         string
	workerType   string
	pollInterval time.Duration
	httpClient   *http.Client
	registeredID string
}

func NewClient(serverURL, workerToken, name, workerType string, pollInterval time.Duration) *Client {
	return &Client{
		serverURL:    strings.TrimRight(serverURL, "/"),
		workerToken:  workerToken,
		name:         name,
		workerType:   workerType,
		pollInterval: pollInterval,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func (c *Client) Run() error {
	worker, err := c.register()
	if err != nil {
		return err
	}

	c.registeredID = worker.ID

	heartbeatTicker := time.NewTicker(10 * time.Second)
	defer heartbeatTicker.Stop()

	pollTicker := time.NewTicker(c.pollInterval)
	defer pollTicker.Stop()

	// 轮询去处理 jobs
	for {
		select {
		case <-heartbeatTicker.C:
			if err := c.heartbeat(); err != nil {
				return err
			}
		case <-pollTicker.C:
			job, ok, err := c.poll()
			if err != nil {
				return err
			}
			if !ok {
				continue
			}
			if err := c.complete(job.ID, true, "phase1 simulated execution completed"); err != nil {
				return err
			}
		}
	}
}

func (c *Client) register() (model.Worker, error) {
	var worker model.Worker
	err := c.postJSON("/api/v1/workers/register", map[string]string{
		"name":        c.name,
		"worker_type": c.workerType,
	}, &worker)
	return worker, err
}

func (c *Client) heartbeat() error {
	path := fmt.Sprintf("/api/v1/workers/%s/heartbeat", c.registeredID)
	return c.postJSON(path, map[string]string{}, nil)
}

func (c *Client) poll() (model.Job, bool, error) {
	var response struct {
		Job *model.Job `json:"job"`
	}

	path := fmt.Sprintf("/api/v1/workers/%s/poll", c.registeredID)
	if err := c.postJSON(path, map[string]string{}, &response); err != nil {
		return model.Job{}, false, err
	}

	if response.Job == nil {
		return model.Job{}, false, nil
	}

	return *response.Job, true, nil
}

func (c *Client) complete(jobID string, success bool, output string) error {
	path := fmt.Sprintf("/api/v1/workers/%s/jobs/%s/complete", c.registeredID, jobID)
	return c.postJSON(path, map[string]any{
		"success": success,
		"output":  output,
	}, nil)
}

func (c *Client) postJSON(path string, body any, target any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, c.serverURL+path, bytes.NewReader(payload))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Worker-Token", c.workerToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		var apiErr map[string]string
		_ = json.NewDecoder(resp.Body).Decode(&apiErr)
		return fmt.Errorf("request failed: %s", apiErr["error"])
	}

	if target != nil {
		return json.NewDecoder(resp.Body).Decode(target)
	}

	return nil
}
