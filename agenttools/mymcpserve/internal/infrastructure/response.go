package infrastructure

import (
	"encoding/json"
	"errors"
	"time"
)

const responseSchemaVersion = "1"

type responseEnvelope struct {
	Data  any            `json:"data"`
	Meta  map[string]any `json:"meta"`
	Error *responseError `json:"error"`
}

type responseError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}

type responseErrorDescriptor interface {
	Code() string
	Retryable() bool
}

func responseSchema(dataSchema any) map[string]any {
	if dataSchema == nil {
		dataSchema = map[string]any{}
	}

	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"data", "meta", "error"},
		"properties": map[string]any{
			"data": map[string]any{
				"anyOf": []any{
					map[string]any{"type": "null"},
					dataSchema,
				},
			},
			"meta": map[string]any{
				"type":                 "object",
				"additionalProperties": true,
			},
			"error": map[string]any{
				"anyOf": []any{
					map[string]any{"type": "null"},
					map[string]any{
						"type":     "object",
						"required": []string{"code", "message", "retryable"},
						"properties": map[string]any{
							"code":      map[string]any{"type": "string"},
							"message":   map[string]any{"type": "string"},
							"retryable": map[string]any{"type": "boolean"},
						},
					},
				},
			},
		},
	}
}

func newSuccessResult(toolName string, data, handlerMeta any, requestedAt time.Time) map[string]any {
	meta, err := buildResponseMeta(toolName, handlerMeta, "success", requestedAt)
	if err != nil {
		return newErrorEnvelope(toolName, nil, "INTERNAL_ERROR", "failed to encode tool result", false, requestedAt)
	}

	envelope := responseEnvelope{Data: data, Meta: meta}
	if _, err := json.Marshal(envelope); err != nil {
		return newErrorEnvelope(toolName, nil, "INTERNAL_ERROR", "failed to encode tool result", false, requestedAt)
	}
	return map[string]any{
		"data":  envelope.Data,
		"meta":  envelope.Meta,
		"error": nil,
	}
}

func newErrorEnvelope(toolName string, handlerMeta any, code, message string, retryable bool, requestedAt time.Time) map[string]any {
	meta, err := buildResponseMeta(toolName, handlerMeta, "error", requestedAt)
	if err != nil {
		meta, _ = buildResponseMeta(toolName, nil, "error", requestedAt)
	}

	return map[string]any{
		"data": nil,
		"meta": meta,
		"error": &responseError{
			Code:      code,
			Message:   message,
			Retryable: retryable,
		},
	}
}

func buildResponseMeta(toolName string, handlerMeta any, executionStatus string, requestedAt time.Time) (map[string]any, error) {
	meta := make(map[string]any)
	if handlerMeta != nil {
		encoded, err := json.Marshal(handlerMeta)
		if err != nil {
			return nil, err
		}

		var value any
		if err := json.Unmarshal(encoded, &value); err != nil {
			return nil, err
		}
		if object, ok := value.(map[string]any); ok {
			for key, item := range object {
				meta[key] = item
			}
		} else {
			// Keep the envelope's meta field an object even when an older handler
			// returns scalar metadata.
			meta["details"] = value
		}
	}

	meta["schema_version"] = responseSchemaVersion
	meta["tool"] = toolName
	if requestedAt.IsZero() {
		requestedAt = time.Now().UTC()
	}
	meta["requested_at"] = requestedAt.UTC().Format(time.RFC3339Nano)
	meta["execution_status"] = executionStatus
	if executionStatus == "error" {
		meta["data_status"] = "error"
	} else if dataStatus, ok := meta["data_status"].(string); !ok || dataStatus == "" {
		meta["data_status"] = "unknown"
	}
	if warnings, ok := meta["warnings"]; !ok || warnings == nil {
		meta["warnings"] = []any{}
	}
	return meta, nil
}

func describeToolError(err error) (code string, retryable bool) {
	code = "TOOL_ERROR"
	if err == nil {
		return code, false
	}

	var descriptor responseErrorDescriptor
	if errors.As(err, &descriptor) {
		if descriptor.Code() != "" {
			code = descriptor.Code()
		}
		return code, descriptor.Retryable()
	}
	return code, false
}
