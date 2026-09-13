package config

import (
	"errors"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"
)

// This file handles the user-agent regexp stored in block_ua_list.list: it
// reads the regexp, adds user agents to it, and builds it from a list of user
// agents.

// maxUserAgents is the largest number of user agents allowed in the
// block_ua_list.list regular expression. The expression is matched against
// every incoming request, and each stored user agent adds work to that match.
// The limit keeps the cost per request small while leaving room for local
// additions.
const maxUserAgents = 250

// ErrTooManyUserAgents is returned when block_ua_list.list holds more than
// maxUserAgents user agents.
var ErrTooManyUserAgents = errors.New("too many user agents")

// ErrEmptyUserAgents is returned when no user agents are given.
var ErrEmptyUserAgents = errors.New("no user agents")

// parseUserAgentRegexp returns the user agents stored in regExpr as a set. The value must have
// the shape "(agent1|agent2|...)", the shape BuildUserAgentRegexp produces:
// the parentheses group the user agents and "|" separates them. Each key is
// returned as it is stored, still escaped with regexp.QuoteMeta.
func parseUserAgentRegexp(regExpr string) (map[string]struct{}, error) {
	shapeErr := validateUserAgentRegexpHasParentheses(regExpr)
	if shapeErr != nil {
		return nil, shapeErr
	}

	stored := make(map[string]struct{})

	inner := regExpr[1 : len(regExpr)-1]
	if inner == "" {
		return stored, nil
	}

	for _, agent := range strings.Split(inner, "|") {
		stored[agent] = struct{}{}
	}

	return stored, nil
}

// validateUserAgentRegexpHasParentheses rejects expressions not wrapped in
// parentheses.
func validateUserAgentRegexpHasParentheses(regExpr string) error {
	if len(regExpr) < 2 || regExpr[0] != '(' || regExpr[len(regExpr)-1] != ')' {
		return fmt.Errorf("user-agent regexp must be wrapped in parentheses, got %q", regExpr)
	}

	return nil
}

// quoteUserAgents escapes each user agent with regexp.QuoteMeta, so characters
// that have a special meaning in a regular expression, such as "." or "+",
// match as plain text.
func quoteUserAgents(agents []string) ([]string, error) {
	quotedAgents := make([]string, 0, len(agents))

	for _, agent := range agents {
		if agent == "" {
			return nil, fmt.Errorf("user agent must not be empty")
		}

		quotedAgents = append(quotedAgents, regexp.QuoteMeta(agent))
	}

	return quotedAgents, nil
}

// validateUserAgentCount rejects more than maxUserAgents user agents.
func validateUserAgentCount(count int) error {
	if count > maxUserAgents {
		return fmt.Errorf("%w: %d agents (max %d)", ErrTooManyUserAgents, count, maxUserAgents)
	}

	return nil
}

// validateUserAgentRegexp checks regExpr with Regexp.UnmarshalText, so an
// invalid expression is never stored.
func validateUserAgentRegexp(regExpr string) error {
	var compiled Regexp
	err := compiled.UnmarshalText([]byte(regExpr))
	if err != nil {
		return err
	}

	return nil
}

// BuildUserAgentRegexp builds the block_ua_list.list regular expression from
// user agents, turning ["GPTBot", "SemrushBot"] into "(GPTBot|SemrushBot)".
// Each user agent is escaped with regexp.QuoteMeta, so characters that have a
// special meaning in a regular expression, such as "." or "+", match as plain
// text. The result is validated with Regexp.UnmarshalText, so an invalid
// expression is never returned, and the number of user agents is capped at
// maxUserAgents.
func BuildUserAgentRegexp(agents []string) (string, error) {
	if len(agents) == 0 {
		return "", ErrEmptyUserAgents
	}

	validateErr := validateUserAgentCount(len(agents))
	if validateErr != nil {
		return "", validateErr
	}

	quotedAgents, err := quoteUserAgents(agents)
	if err != nil {
		return "", err
	}

	regExpr := "(" + strings.Join(quotedAgents, "|") + ")"

	validateErr = validateUserAgentRegexp(regExpr)
	if validateErr != nil {
		return "", validateErr
	}

	return regExpr, nil
}

// AddUserAgents adds user agents to the block_ua_list.list regular expression.
// Each user agent is escaped with regexp.QuoteMeta, membership is checked
// against the stored set, and the rebuilt expression is validated with
// Regexp.UnmarshalText before it is returned. Adding user agents that are
// already stored leaves regExpr unchanged.
func AddUserAgents(regExpr string, agents ...string) (string, error) {
	newAgents, err := quoteUserAgents(agents)
	if err != nil {
		return "", err
	}

	var stored map[string]struct{}
	stored, err = parseUserAgentRegexp(regExpr)
	if err != nil {
		return "", err
	}

	added := false

	for _, newAgent := range newAgents {
		_, ok := stored[newAgent]
		if !ok {
			stored[newAgent] = struct{}{}
			added = true
		}
	}

	if !added {
		return regExpr, nil
	}

	validateErr := validateUserAgentCount(len(stored))
	if validateErr != nil {
		return "", validateErr
	}

	listed := slices.Collect(maps.Keys(stored))

	regExpr = "(" + strings.Join(listed, "|") + ")"

	validateErr = validateUserAgentRegexp(regExpr)
	if validateErr != nil {
		return "", validateErr
	}

	return regExpr, nil
}
