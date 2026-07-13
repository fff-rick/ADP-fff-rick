package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"adp/internal/domain/model"
)

type Client struct {
	serverURL    string
	workerToken  string
	name         string
	workerType   string
	pollInterval time.Duration
	httpClient   *http.Client
	registeredID string
	execTimeout  time.Duration
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
		execTimeout: 30 * time.Second,
	}
}

func (c *Client) Run() error {
	worker, err := c.register()
	if err != nil {
		return err
	}

	c.registeredID = worker.ID
	log.Printf("level=INFO component=worker action=register worker_id=%s worker_name=%s worker_type=%s", worker.ID, worker.Name, worker.WorkerType)

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
			log.Printf("level=INFO component=worker action=job_polled worker_id=%s job_id=%s", c.registeredID, job.ID)
			output, success := c.executeCommand(job.Command)
			if err := c.complete(job.ID, success, output); err != nil {
				return err
			}
			log.Printf("level=INFO component=worker action=job_completed worker_id=%s job_id=%s success=%t", c.registeredID, job.ID, success)
		}
	}
}

// executeCommand runs a shell command with timeout and returns combined output.
func (c *Client) executeCommand(cmd string) (string, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), c.execTimeout)
	defer cancel()

	out, err := exec.CommandContext(ctx, "sh", "-c", cmd).CombinedOutput()
	if err != nil {
		return fmt.Sprintf("exit_error: %v\n%s", err, string(out)), false
	}
	return string(out), true
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
	defer resp.Body.Close() //nolint:errcheck

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
