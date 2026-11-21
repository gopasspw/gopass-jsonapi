package jsonapi

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
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

// trickleReader returns its content a few bytes at a time, like a pipe that
// delivers a message in pieces.
type trickleReader struct {
	data []byte
}

func (r *trickleReader) Read(p []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, io.EOF
	}

	n := min(len(p), len(r.data), 7)
	copy(p, r.data[:n])
	r.data = r.data[n:]

	return n, nil
}

func TestReadRequest(t *testing.T) {
	t.Parallel()

	msg := `{"type": "query", "query": "` + strings.Repeat("x", 20000) + `"}`
	framed := []byte(writeMessageWithLength(msg))

	// A message does not have to arrive in one piece.
	got, err := readRequest(&trickleReader{data: framed})
	require.NoError(t, err)
	assert.Equal(t, msg, string(got))

	got, err = readRequest(bytes.NewReader(framed))
	require.NoError(t, err)
	assert.Equal(t, msg, string(got))

	// a message that is cut off
	for _, cut := range []int{1, 3, 4, 5, 100, len(framed) - 1} {
		got, err = readRequest(&trickleReader{data: framed[:cut]})
		require.Error(t, err, cut)
		assert.Nil(t, got, cut)
	}

	// no message at all is not an error
	got, err = readRequest(&trickleReader{})
	require.NoError(t, err)
	assert.Nil(t, got)

	// an empty message
	got, err = readRequest(strings.NewReader("\x00\x00\x00\x00"))
	require.NoError(t, err)
	assert.Empty(t, got)

	// A length that is not plausible is refused before anything is read.
	got, err = readRequest(strings.NewReader("\xff\xff\xff\x7f{}"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "too large")
	assert.Nil(t, got)
}
