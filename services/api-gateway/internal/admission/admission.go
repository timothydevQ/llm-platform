// Package admission validates inference requests and enforces admission
// controls (max tokens, deadline, body size) before they reach the router.
package admission

import (
	"fmt"
	"time"

	inferencev1 "github.com/timothydevQ/llm-platform/gen/inference/v1"
)

// Config controls the admission policy limits.
type Config struct {
	MaxPromptBytes  int   // hard limit on prompt + messages bytes
	MaxTokens       int32 // hard cap on max_tokens
	MaxDocuments    int
	DeadlineDefault time.Duration // if client omits deadline
	DeadlineMax     time.Duration
}

func DefaultConfig() Config {
	return Config{
		MaxPromptBytes:  128 * 1024, // 128KB
		MaxTokens:       8192,
		MaxDocuments:    50,
		DeadlineDefault: 30 * time.Second,
		DeadlineMax:     120 * time.Second,
	}
}

// Admission enforces request-level controls.
type Admission struct {
	cfg Config
}

func New(cfg Config) *Admission {
	return &Admission{cfg: cfg}
}

// Admit validates the request and returns a normalised copy with defaults filled
// in. Returns an error if any hard limit is exceeded.
func (a *Admission) Admit(req *inferencev1.InferenceRequest) error {
	if err := a.validateTask(req); err != nil {
		return err
	}
	if err := a.validateSize(req); err != nil {
		return err
	}
	if err := a.validateTokens(req); err != nil {
		return err
	}
	if err := a.validateDocuments(req); err != nil {
		return err
	}
	a.normalise(req)
	return nil
}

func (a *Admission) validateTask(req *inferencev1.InferenceRequest) error {
	switch req.TaskType {
	case inferencev1.TaskChat, inferencev1.TaskSummarize:
		if len(req.Messages) == 0 && req.Prompt == "" {
			return fmt.Errorf("task %s requires prompt or messages", req.TaskType)
		}
	case inferencev1.TaskEmbed:
		if req.Prompt == "" && req.Query == "" {
			return fmt.Errorf("embed requires prompt or query")
		}
	case inferencev1.TaskRerank:
		if req.Query == "" {
			return fmt.Errorf("rerank requires query")
		}
		if len(req.Documents) == 0 {
			return fmt.Errorf("rerank requires documents")
		}
	case inferencev1.TaskClassify, inferencev1.TaskModerate:
		if req.Prompt == "" {
			return fmt.Errorf("task %s requires prompt", req.TaskType)
		}
	case inferencev1.TaskUnspecified:
		return fmt.Errorf("task_type must be specified")
	}
	return nil
}

func (a *Admission) validateSize(req *inferencev1.InferenceRequest) error {
	total := len(req.Prompt) + len(req.Query)
	for _, m := range req.Messages {
		total += len(m.Content)
	}
	for _, d := range req.Documents {
		total += len(d)
	}
	if total > a.cfg.MaxPromptBytes {
		return fmt.Errorf("request body exceeds %d bytes (got %d)", a.cfg.MaxPromptBytes, total)
	}
	return nil
}

func (a *Admission) validateTokens(req *inferencev1.InferenceRequest) error {
	if req.MaxTokens < 0 {
		return fmt.Errorf("max_tokens cannot be negative")
	}
	if req.MaxTokens > a.cfg.MaxTokens {
		return fmt.Errorf("max_tokens %d exceeds limit %d", req.MaxTokens, a.cfg.MaxTokens)
	}
	return nil
}

func (a *Admission) validateDocuments(req *inferencev1.InferenceRequest) error {
	if len(req.Documents) > a.cfg.MaxDocuments {
		return fmt.Errorf("too many documents: %d (max %d)", len(req.Documents), a.cfg.MaxDocuments)
	}
	return nil
}

func (a *Admission) normalise(req *inferencev1.InferenceRequest) {
	// Apply default max_tokens
	if req.MaxTokens == 0 {
		req.MaxTokens = 1024
	}
	// Apply default / cap deadline
	now := time.Now()
	if req.DeadlineUnixMs == 0 {
		req.DeadlineUnixMs = now.Add(a.cfg.DeadlineDefault).UnixMilli()
	} else {
		deadline := time.UnixMilli(req.DeadlineUnixMs)
		maxDeadline := now.Add(a.cfg.DeadlineMax)
		if deadline.After(maxDeadline) {
			req.DeadlineUnixMs = maxDeadline.UnixMilli()
		}
	}
	// Initialise metadata map if nil
	if req.Metadata == nil {
		req.Metadata = make(map[string]string)
	}
}

// DeadlineRemaining returns how much time remains before the request deadline.
func DeadlineRemaining(req *inferencev1.InferenceRequest) time.Duration {
	if req.DeadlineUnixMs == 0 {
		return 30 * time.Second
	}
	return time.Until(time.UnixMilli(req.DeadlineUnixMs))
}
// tw_6059_23342
// tw_6059_1686
