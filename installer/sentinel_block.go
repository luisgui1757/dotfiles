package installer

import (
	"bytes"
	"errors"
	"strings"
)

const sentinelBegin = "<!-- AGENT-RULES:BEGIN do-not-edit-inside-this-block -->"
const sentinelEnd = "<!-- AGENT-RULES:END -->"

// Accept one complete upstream block, including earlier versions, so explicit
// adoption can preserve its exact bytes as the removal baseline. Ambiguous or
// damaged markers never authorize replacing policy or surrounding prose.
func sentinelBlock(data []byte) (start, end int, err error) {
	encode, err := sentinelBlockEncoding(data)
	if err != nil {
		return 0, 0, err
	}
	beginToken, endToken := encode("<!-- AGENT-RULES:BEGIN"), encode("<!-- AGENT-RULES:END")
	begins, ends := bytes.Count(data, beginToken), bytes.Count(data, endToken)
	if begins == 0 && ends == 0 {
		return -1, -1, nil
	}
	if begins != 1 || ends != 1 {
		return 0, 0, errors.New("Sentinel policy markers are duplicated or incomplete; preserve the policy")
	}
	start = bytes.Index(data, beginToken)
	prefix := data[:start]
	if len(prefix) != 0 && !bytes.Equal(prefix, []byte{0xef, 0xbb, 0xbf}) && !bytes.Equal(prefix, []byte{0xff, 0xfe}) && !bytes.Equal(prefix, []byte{0xfe, 0xff}) && !bytes.HasSuffix(prefix, encode("\n")) {
		return 0, 0, errors.New("Sentinel opening marker is not on its own line")
	}
	body := -1
	for _, opening := range []string{sentinelBegin, "<!-- AGENT-RULES:BEGIN -->"} {
		for _, nl := range []string{"\r\n", "\n"} {
			marker := encode(opening + nl)
			if bytes.HasPrefix(data[start:], marker) {
				body = start + len(marker)
			}
		}
	}
	closing := bytes.Index(data, endToken)
	if body < 0 || closing < body || !bytes.HasSuffix(data[:closing], encode("\n")) || !bytes.HasPrefix(data[closing:], encode(sentinelEnd)) {
		return 0, 0, errors.New("Sentinel policy markers were edited; preserve the policy")
	}
	end = closing + len(encode(sentinelEnd))
	if end != len(data) {
		if bytes.HasPrefix(data[end:], encode("\r\n")) {
			end += len(encode("\r\n"))
		} else if bytes.HasPrefix(data[end:], encode("\n")) {
			end += len(encode("\n"))
		} else {
			return 0, 0, errors.New("Sentinel closing marker is not on its own line")
		}
	}
	// The separator we add is part of our block, not personal text. Keeping it
	// in adopted baselines also restores pre-existing upstream bytes exactly.
	if bytes.HasSuffix(prefix, encode("\r\n")) {
		start -= len(encode("\r\n"))
	} else if bytes.HasSuffix(prefix, encode("\n")) {
		start -= len(encode("\n"))
	}
	return start, end, nil
}

func renderSentinelBlock(data []byte, policy string) ([]byte, error) {
	if strings.ContainsAny(policy, "\x00\r") {
		return nil, errors.New("invalid generated Sentinel policy")
	}
	start, end, err := sentinelBlock([]byte(policy))
	if err != nil || start != 0 || end != len(policy) || !strings.HasPrefix(policy, sentinelBegin+"\n") || !strings.HasSuffix(policy, sentinelEnd+"\n") {
		return nil, errors.Join(errors.New("Sentinel recipe must contain exactly its generated policy block"), err)
	}
	encode, nl, err := profileEncoding(data)
	if err != nil {
		return nil, err
	}
	start, _, err = sentinelBlock(data)
	if err != nil {
		return nil, err
	}
	separator := ""
	if start < 0 && len(data) != 0 || start >= 0 && bytes.HasPrefix(data[start:], encode("\n")) || start >= 0 && bytes.HasPrefix(data[start:], encode("\r\n")) {
		separator = nl
	}
	return encode(separator + strings.ReplaceAll(policy, "\n", nl)), nil
}

// Profile journals retain delimited block bytes without the surrounding file
// BOM. Recognize only a marked UTF-16 fragment in that case, and still validate
// every code unit. Full consumer files must pass readProfile's BOM requirement.
func sentinelBlockEncoding(data []byte) (func(string) []byte, error) {
	encode, _, err := profileEncoding(data)
	if err == nil {
		return encode, nil
	}
	for _, bom := range [][]byte{{0xff, 0xfe}, {0xfe, 0xff}} {
		candidate, _, bomErr := profileEncoding(bom)
		if bomErr != nil {
			return nil, bomErr
		}
		for _, separator := range []string{"", "\n", "\r\n"} {
			if bytes.HasPrefix(data, candidate(separator+"<!-- AGENT-RULES:BEGIN")) {
				encode, _, err = profileEncoding(append(bytes.Clone(bom), data...))
				return encode, err
			}
		}
	}
	return nil, err
}
