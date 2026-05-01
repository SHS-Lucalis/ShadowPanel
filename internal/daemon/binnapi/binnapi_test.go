package binnapi

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/et-nik/binngo"
	"github.com/et-nik/binngo/decode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testMarshaler implements the encode.Marshaler interface by delegating to
// binngo.Marshal on a payload slice. WriteMessage will call MarshalBINN on the
// passed value, so this exercises the same code path as production callers.
type testMarshaler struct {
	payload []any
}

func (m *testMarshaler) MarshalBINN() ([]byte, error) {
	return binngo.Marshal(&m.payload)
}

// listUnmarshaler is a decode.Unmarshaler that captures the raw BINN container
// bytes so tests can pass the bytes to binngo.Unmarshal afterward.
type listUnmarshaler struct {
	values []any
}

func (u *listUnmarshaler) UnmarshalBINN(b []byte) error {
	return binngo.Unmarshal(b, &u.values)
}

func TestWriteMessage_appendsEndBytes(t *testing.T) {
	// ARRANGE
	var buf bytes.Buffer
	m := &testMarshaler{payload: []any{uint8(7), "hello", uint8(42)}}

	// ACT
	err := WriteMessage(&buf, m)

	// ASSERT
	require.NoError(t, err)

	written := buf.Bytes()
	require.GreaterOrEqual(t, len(written), len(DaemonBinnEndBytes), "written buffer must include end sentinel")

	tail := written[len(written)-len(DaemonBinnEndBytes):]
	assert.Equal(t, DaemonBinnEndBytes, tail, "last 4 bytes must equal DaemonBinnEndBytes")

	prefix := written[:len(written)-len(DaemonBinnEndBytes)]
	var decoded []any
	require.NoError(t, binngo.Unmarshal(prefix, &decoded), "prefix bytes must be valid BINN")
	require.Len(t, decoded, 3)
	assert.Equal(t, "hello", decoded[1])
}

func TestReadMessage_decodesAndConsumesEndBytes(t *testing.T) {
	// ARRANGE
	payload := []any{uint8(1), "world", uint8(99)}
	encoded, err := binngo.Marshal(&payload)
	require.NoError(t, err)

	buf := bytes.NewBuffer(nil)
	buf.Write(encoded)
	buf.Write(DaemonBinnEndBytes)

	var got listUnmarshaler

	// ACT
	err = ReadMessage(buf, &got)

	// ASSERT
	require.NoError(t, err)
	require.Len(t, got.values, 3)
	assert.Equal(t, "world", got.values[1], "decoded payload must match original")
	assert.Equal(t, 0, buf.Len(), "ReadMessage must fully consume the end-bytes sentinel")
}

func TestReadMessage_invalidEndBytes_returnsError(t *testing.T) {
	// ARRANGE
	payload := []any{uint8(1), "x"}
	encoded, err := binngo.Marshal(&payload)
	require.NoError(t, err)

	buf := bytes.NewBuffer(nil)
	buf.Write(encoded)
	buf.Write([]byte{0xFF, 0xFF, 0xFF, 0x00})

	var got listUnmarshaler

	// ACT
	err = ReadMessage(buf, &got)

	// ASSERT
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid message end bytes", "must surface invalid sentinel error")
	assert.True(t, errors.Is(err, ErrInvalidEndBytes), "errors.Is must match ErrInvalidEndBytes sentinel")
}

func TestReadMessage_truncatedEndBytes_returnsError(t *testing.T) {
	// ARRANGE
	payload := []any{uint8(1), "y"}
	encoded, err := binngo.Marshal(&payload)
	require.NoError(t, err)

	full := bytes.NewBuffer(nil)
	full.Write(encoded)
	full.Write([]byte{0xFF, 0xFF, 0xFF})

	reader := bytes.NewReader(full.Bytes())

	var got listUnmarshaler

	// ACT
	err = ReadMessage(reader, &got)

	// ASSERT
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid message end bytes")
	assert.True(t, errors.Is(err, ErrInvalidEndBytes))
}

func TestReadEndBytes_eof_returnsNil(t *testing.T) {
	// ARRANGE
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	t.Cleanup(cancel)

	reader := bytes.NewReader(nil)

	// ACT
	err := ReadEndBytes(ctx, reader)

	// ASSERT
	require.NoError(t, err, "EOF on empty reader must be treated as a non-error")
}

func TestReadEndBytes_validBytes_ok(t *testing.T) {
	// ARRANGE
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	t.Cleanup(cancel)

	reader := bytes.NewReader(DaemonBinnEndBytes)

	// ACT
	err := ReadEndBytes(ctx, reader)

	// ASSERT
	require.NoError(t, err)
}

func TestReadEndBytes_contextCanceled_returnsCtxErr(t *testing.T) {
	// ARRANGE
	pipeReader, pipeWriter := io.Pipe()
	t.Cleanup(func() {
		_ = pipeReader.Close()
		_ = pipeWriter.Close()
	})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	t.Cleanup(cancel)

	// ACT
	err := ReadEndBytes(ctx, pipeReader)

	// ASSERT
	require.Error(t, err)
	assert.True(t, errors.Is(err, context.DeadlineExceeded), "context error must propagate when reader blocks")
}

func TestReadMessageToSlice_roundTrip(t *testing.T) {
	// ARRANGE
	payload := []any{uint8(1), "x", uint8(3)}
	encoded, err := binngo.Marshal(&payload)
	require.NoError(t, err)

	buf := bytes.NewBuffer(nil)
	buf.Write(encoded)
	buf.Write(DaemonBinnEndBytes)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	t.Cleanup(cancel)

	// ACT
	msgs, err := ReadMessageToSlice(ctx, buf)

	// ASSERT
	require.NoError(t, err)
	require.Len(t, msgs, 3)
	assert.Equal(t, "x", msgs[1], "decoded slice must round-trip the original payload")
	assert.Equal(t, 0, buf.Len(), "buffer must be fully consumed including the sentinel")
}

func TestReadMessageToSlice_invalidEndBytes_returnsError(t *testing.T) {
	// ARRANGE
	payload := []any{uint8(1), "x"}
	encoded, err := binngo.Marshal(&payload)
	require.NoError(t, err)

	buf := bytes.NewBuffer(nil)
	buf.Write(encoded)
	buf.Write([]byte{0xFF, 0x00, 0xFF, 0x00})

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	t.Cleanup(cancel)

	// ACT
	msgs, err := ReadMessageToSlice(ctx, buf)

	// ASSERT
	require.Error(t, err)
	assert.Nil(t, msgs, "no messages must be returned when sentinel validation fails")
	assert.Contains(t, err.Error(), "invalid message end bytes")
	assert.True(t, errors.Is(err, ErrInvalidEndBytes))
}

// TestReadMessage_decoderError_returnsWrappedError verifies that decoder
// failures are wrapped with the expected message prefix.
func TestReadMessage_decoderError_returnsWrappedError(t *testing.T) {
	// ARRANGE
	reader := bytes.NewReader([]byte{0xDE, 0xAD, 0xBE, 0xEF})

	var msg listUnmarshaler

	// ACT
	err := ReadMessage(reader, &msg)

	// ASSERT
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode message", "decoder failure must be wrapped")
}

// loadDecoderViaDecode is used as a sanity check for the decoder helper
// signature so other tests can rely on the exact decode.NewDecoder API.
func TestDecoderCompatibilityWithBINN(t *testing.T) {
	// ARRANGE
	payload := []any{uint8(1), "x"}
	encoded, err := binngo.Marshal(&payload)
	require.NoError(t, err)

	var got []any

	// ACT
	err = decode.NewDecoder(bytes.NewReader(encoded)).Decode(&got)

	// ASSERT
	require.NoError(t, err)
	require.Len(t, got, 2)
}
