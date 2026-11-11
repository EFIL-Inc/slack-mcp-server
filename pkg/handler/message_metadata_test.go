package handler

import (
	"encoding/json"
	"testing"

	"github.com/slack-go/slack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestUnitParseMessageMetadata_Absent(t *testing.T) {
	for _, raw := range []any{nil, "", "   "} {
		meta, err := parseMessageMetadata(raw)
		require.NoError(t, err)
		assert.Nil(t, meta)
	}
}

func TestUnitParseMessageMetadata_Object(t *testing.T) {
	raw := map[string]any{
		"event_type":    "latens_fired",
		"event_payload": map[string]any{"run_id": "run-123"},
	}
	meta, err := parseMessageMetadata(raw)
	require.NoError(t, err)
	require.NotNil(t, meta)
	assert.Equal(t, "latens_fired", meta.EventType)
	assert.Equal(t, "run-123", meta.EventPayload["run_id"])
}

func TestUnitParseMessageMetadata_String(t *testing.T) {
	meta, err := parseMessageMetadata(`{"event_type":"latens_fired","event_payload":{"run_id":"run-9"}}`)
	require.NoError(t, err)
	require.NotNil(t, meta)
	assert.Equal(t, "latens_fired", meta.EventType)
	assert.Equal(t, "run-9", meta.EventPayload["run_id"])
}

func TestUnitParseMessageMetadata_PayloadOptional(t *testing.T) {
	meta, err := parseMessageMetadata(`{"event_type":"latens_fired"}`)
	require.NoError(t, err)
	require.NotNil(t, meta)
	assert.Equal(t, "latens_fired", meta.EventType)
	assert.NotNil(t, meta.EventPayload)
}

func TestUnitParseMessageMetadata_Invalid(t *testing.T) {
	cases := map[string]any{
		"not json":           "{not json",
		"array":              `[1,2]`,
		"missing event_type": `{"event_payload":{"a":1}}`,
		"empty event_type":   map[string]any{"event_type": " "},
		"unknown field":      `{"event_type":"x","extra":1}`,
		"payload not object": `{"event_type":"x","event_payload":"str"}`,
		"number":             42,
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			meta, err := parseMessageMetadata(raw)
			assert.Error(t, err)
			assert.Nil(t, meta)
		})
	}
}

// The parsed metadata must reach chat.postMessage as the "metadata" form value,
// through the same option builder the handler uses.
func TestUnitBuildAddMessageOptions_Metadata(t *testing.T) {
	ch := &ConversationsHandler{logger: zap.NewNop()}
	meta, err := parseMessageMetadata(map[string]any{
		"event_type":    "latens_fired",
		"event_payload": map[string]any{"run_id": "run-1"},
	})
	require.NoError(t, err)

	opts, err := ch.buildAddMessageOptions(&addMessageParams{
		channel: "C123", text: "hi", contentType: "text/plain", threadTs: "1.2", metadata: meta,
	})
	require.NoError(t, err)
	_, values, err := slack.UnsafeApplyMsgOptions("tok", "C123", "https://slack.example/api/", opts...)
	require.NoError(t, err)

	var sent map[string]any
	require.NoError(t, json.Unmarshal([]byte(values.Get("metadata")), &sent))
	assert.Equal(t, "latens_fired", sent["event_type"])
	assert.Equal(t, map[string]any{"run_id": "run-1"}, sent["event_payload"])
	assert.Equal(t, "hi", values.Get("text"))
	assert.Equal(t, "1.2", values.Get("thread_ts"))

	// Without metadata nothing is sent (existing callers unchanged).
	opts, err = ch.buildAddMessageOptions(&addMessageParams{
		channel: "C123", text: "hi", contentType: "text/plain",
	})
	require.NoError(t, err)
	_, values, err = slack.UnsafeApplyMsgOptions("tok", "C123", "https://slack.example/api/", opts...)
	require.NoError(t, err)
	assert.Empty(t, values.Get("metadata"))
	assert.Equal(t, "hi", values.Get("text"))
}
