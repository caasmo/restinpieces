package main

import (
	"fmt"

	"github.com/caasmo/restinpieces/config"
)

// This file adds one user agent to the block_ua_list.list regular expression.

// userAgentAdder adds a user agent to the block_ua_list.list regular
// expression.
type userAgentAdder struct{}

func (userAgentAdder) Add(existing interface{}, value string) (interface{}, error) {
	regExpr, ok := existing.(string)
	if !ok {
		return nil, fmt.Errorf("%w: block_ua_list.list is %T, expected string", ErrNotCollection, existing)
	}

	updated, err := config.AddUserAgents(regExpr, value)
	if err != nil {
		return nil, err
	}
	return updated, nil
}
