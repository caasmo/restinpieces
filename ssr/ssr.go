package ssr

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// Marshal injects data as a JSON script tag into the HTML payload before the </head> tag.
// data is enforced as map[string]any to guarantee a stable key-based contract with the client SDK.
// nonce is optional; if empty, the nonce attribute is omitted from the script tag.
func Marshal(htmlPayload []byte, data map[string]any, nonce string) ([]byte, error) {
	encoded, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("ssr.Marshal: failed to encode data: %w", err)
	}

	var script []byte
	if nonce != "" {
		script = fmt.Appendf(nil, `<script nonce="%s">window.__INITIAL_DATA__=%s;</script></head>`, nonce, encoded)
	} else {
		script = fmt.Appendf(nil, `<script>window.__INITIAL_DATA__=%s;</script></head>`, encoded)
	}

	result := bytes.Replace(htmlPayload, []byte("</head>"), script, 1)
	if bytes.Equal(result, htmlPayload) {
		return nil, fmt.Errorf("ssr.Marshal: </head> tag not found in payload")
	}

	return result, nil
}
