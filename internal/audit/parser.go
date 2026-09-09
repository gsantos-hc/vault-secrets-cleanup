// Copyright IBM Corp. 2026
// SPDX-License-Identifier: MIT

package audit

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

type AuditEvent struct {
	Time     time.Time
	Type     string
	Request  RequestInfo
	Auth     AuthInfo
	Response ResponseInfo
}

type RequestInfo struct {
	ID            string
	Operation     string
	Path          string
	Namespace     NamespaceInfo
	MountAccessor string
	MountType     string
}

type NamespaceInfo struct {
	ID   string
	Path string
}

type AuthInfo struct {
	TokenType string
	Policies  []string
}

type ResponseInfo struct {
	MountType string
}

type Parser struct {
	bufferSize int
}

type EventHandler func(AuditEvent) error

func NewParser() *Parser {
	return &Parser{bufferSize: 1024 * 1024}
}

func (p *Parser) Parse(ctx context.Context, reader io.Reader, handler EventHandler) error {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), p.bufferSize)

	lineNum := 0
	for scanner.Scan() {
		lineNum++

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var raw map[string]any
		if err := json.Unmarshal(line, &raw); err != nil {
			continue
		}

		event := parseEvent(raw)
		if err := handler(event); err != nil {
			return fmt.Errorf("handler error at line %d: %w", lineNum, err)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scanner error: %w", err)
	}

	return nil
}

func parseEvent(raw map[string]any) AuditEvent {
	event := AuditEvent{}
	if timeStr, ok := raw["time"].(string); ok {
		if ts, err := time.Parse(time.RFC3339, timeStr); err == nil {
			event.Time = ts
		}
	}
	if typ, ok := raw["type"].(string); ok {
		event.Type = typ
	}
	if req, ok := raw["request"].(map[string]any); ok {
		event.Request = parseRequest(req)
	}
	if auth, ok := raw["auth"].(map[string]any); ok {
		event.Auth = parseAuth(auth)
	}
	return event
}

func parseRequest(raw map[string]any) RequestInfo {
	info := RequestInfo{}
	if id, ok := raw["id"].(string); ok {
		info.ID = id
	}
	if operation, ok := raw["operation"].(string); ok {
		info.Operation = operation
	}
	if path, ok := raw["path"].(string); ok {
		info.Path = path
	}
	if accessor, ok := raw["mount_accessor"].(string); ok {
		info.MountAccessor = accessor
	}
	if mountType, ok := raw["mount_type"].(string); ok {
		info.MountType = mountType
	}
	if namespace, ok := raw["namespace"].(map[string]any); ok {
		if nsID, ok := namespace["id"].(string); ok {
			info.Namespace.ID = nsID
		}
		if nsPath, ok := namespace["path"].(string); ok {
			info.Namespace.Path = nsPath
		}
	}
	return info
}

func parseAuth(raw map[string]any) AuthInfo {
	info := AuthInfo{}
	if tokenType, ok := raw["token_type"].(string); ok {
		info.TokenType = tokenType
	}
	if policies, ok := raw["policies"].([]any); ok {
		for _, p := range policies {
			if policy, ok := p.(string); ok {
				info.Policies = append(info.Policies, policy)
			}
		}
	}
	return info
}
