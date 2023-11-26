package jsonapi

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRoundTrip(t *testing.T) {
	t.Parallel()

	var receivedMessage queryMessage

	message := queryMessage{Query: "holla"}
	var buffer bytes.Buffer

	err := sendResponse(message, &buffer)
	require.NoError(t, err)

	received, err := readRequest(&buffer)
	require.NoError(t, err)

	require.NoError(t, json.Unmarshal(received, &receivedMessage))
	assert.Equal(t, message.Query, receivedMessage.Query)
}
