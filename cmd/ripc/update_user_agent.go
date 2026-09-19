package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// This file downloads the upstream user-agent list that fills block_user_agent.agents.

// Errors returned by the download.
var (
	ErrUserAgentFetch = errors.New("failed to fetch user-agent list")
	ErrUserAgentParse = errors.New("failed to parse user-agent list")
)

// userAgentURL points to robots.json in the ai.robots.txt repository, the
// upstream list of user agents to block.
const userAgentURL = "https://raw.githubusercontent.com/ai-robots-txt/ai.robots.txt/main/robots.json"

// userAgentMaxBytes is the largest download accepted from
// userAgentURL.
const userAgentMaxBytes = 1 << 20 // 1 MiB (1,048,576 bytes)

// fetchUserAgents downloads the user-agent list from url and returns the user
// agents it names. The download gives up after 30 seconds.
func fetchUserAgents(url string) (agents []string, err error) {
	client := &http.Client{Timeout: 30 * time.Second}

	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUserAgentFetch, err)
	}

	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			err = errors.Join(err, fmt.Errorf("%w: failed to close response body: %w", ErrUserAgentFetch, cerr))
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: unexpected status %s", ErrUserAgentFetch, resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, userAgentMaxBytes))
	if err != nil {
		return nil, fmt.Errorf("%w: failed to read response: %w", ErrUserAgentFetch, err)
	}

	var bots map[string]json.RawMessage
	if err := json.Unmarshal(body, &bots); err != nil {
		return nil, fmt.Errorf("%w: failed to parse user-agent list: %w", ErrUserAgentParse, err)
	}

	// Only the keys are user agents. robots.json maps each agent to details:
	// "GPTBot": {
	//   "operator": "[OpenAI](https://openai.com)",
	//   "respect": "Yes",
	//   "function": "Scrapes data to train OpenAI's products.",
	//   "frequency": "No information.",
	//   "description": "Data is used to train current and future models, removed paywalled data, PII and data that violates the company's policies."
	// }.
	for agent := range bots {
		agents = append(agents, agent)
	}

	return agents, nil
}
