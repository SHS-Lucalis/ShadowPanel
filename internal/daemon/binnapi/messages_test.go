package binnapi

import (
	"bytes"
	"errors"
	"testing"

	"github.com/et-nik/binngo"
	"github.com/et-nik/binngo/decode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBaseResponseMessage_MarshalUnmarshal_roundTrip(t *testing.T) {
	tests := []struct {
		name string
		code StatusCode
		info string
	}{
		{name: "status_error", code: StatusCodeError, info: "an error occurred"},
		{name: "status_critical_error", code: StatusCodeCriticalError, info: "panic"},
		{name: "status_unknown_command", code: StatusCodeUnknownCommand, info: "unknown command"},
		{name: "status_ok", code: StatusCodeOK, info: "ok"},
		{name: "status_ready_to_transfer", code: StatusCodeReadyToTransfer, info: "ready"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// ARRANGE
			original := BaseResponseMessage{Code: tt.code, Info: tt.info}

			// ACT
			encoded, err := binngo.Marshal(&original)
			require.NoError(t, err)

			var decoded BaseResponseMessage
			err = decode.NewDecoder(bytes.NewReader(encoded)).Decode(&decoded)

			// ASSERT
			require.NoError(t, err)
			assert.Equal(t, tt.code, decoded.Code, "status code must round-trip unchanged")
			assert.Equal(t, tt.info, decoded.Info, "info string must round-trip unchanged")
		})
	}
}

func TestBaseResponseMessage_FillFromSlice_unsupportedStatusType_returnsError(t *testing.T) {
	// ARRANGE — FillFromSlice's type switch rejects anything that is not
	// uint8/uint16/uint32. A plain int is the simplest representative.
	var msg BaseResponseMessage

	// ACT
	err := msg.FillFromSlice([]any{int(100), "some info"})

	// ASSERT
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrUnknownBINNValue), "non-supported status numeric type must yield ErrUnknownBINNValue")
}

func TestBaseResponseMessage_UnmarshalBINN_supportsVariantNumericTypes(t *testing.T) {
	tests := []struct {
		name string
		raw  []any
		want StatusCode
	}{
		{name: "uint8_ok_value", raw: []any{uint8(100), "ok"}, want: StatusCodeOK},
		{name: "uint16_ok_value", raw: []any{uint16(100), "ok"}, want: StatusCodeOK},
		{name: "uint32_ok_value", raw: []any{uint32(100), "ok"}, want: StatusCodeOK},
		{name: "uint8_error_value", raw: []any{uint8(1), "boom"}, want: StatusCodeError},
		{name: "uint16_error_value", raw: []any{uint16(1), "boom"}, want: StatusCodeError},
		{name: "uint32_error_value", raw: []any{uint32(1), "boom"}, want: StatusCodeError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// ARRANGE
			encoded, err := binngo.Marshal(&tt.raw)
			require.NoError(t, err)

			var msg BaseResponseMessage

			// ACT
			err = msg.UnmarshalBINN(encoded)

			// ASSERT
			require.NoError(t, err)
			assert.Equal(t, tt.want, msg.Code, "status code must normalize across uint8/uint16/uint32")
			assert.Equal(t, tt.raw[1], msg.Info, "info string must be preserved")
		})
	}
}

func TestBaseResponseMessage_UnmarshalBINN_overflowingUint16_returnsError(t *testing.T) {
	// ARRANGE — uint16 over 255 cannot be safely cast to a StatusCode (uint8).
	raw := []any{uint16(300), "info"}
	encoded, err := binngo.Marshal(&raw)
	require.NoError(t, err)

	var msg BaseResponseMessage

	// ACT
	err = msg.UnmarshalBINN(encoded)

	// ASSERT
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrUnknownBINNValue), "uint16 above 255 must surface ErrUnknownBINNValue")
}

func TestBaseResponseMessage_UnmarshalBINN_overflowingUint32_returnsError(t *testing.T) {
	// ARRANGE
	raw := []any{uint32(70000), "info"}
	encoded, err := binngo.Marshal(&raw)
	require.NoError(t, err)

	var msg BaseResponseMessage

	// ACT
	err = msg.UnmarshalBINN(encoded)

	// ASSERT
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrUnknownBINNValue))
}

func TestBaseResponseMessage_UnmarshalBINN_tooFewFields_returnsError(t *testing.T) {
	// ARRANGE
	raw := []any{uint8(100)}
	encoded, err := binngo.Marshal(&raw)
	require.NoError(t, err)

	var msg BaseResponseMessage

	// ACT
	err = msg.UnmarshalBINN(encoded)

	// ASSERT
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrUnknownBINNValue))
}

func TestBaseResponseMessage_UnmarshalBINN_infoNotString_returnsError(t *testing.T) {
	// ARRANGE
	raw := []any{uint8(100), uint8(7)}
	encoded, err := binngo.Marshal(&raw)
	require.NoError(t, err)

	var msg BaseResponseMessage

	// ACT
	err = msg.UnmarshalBINN(encoded)

	// ASSERT
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrUnknownBINNValue))
}

func TestBaseResponseMessage_MarshalBINN_includesData_whenSet(t *testing.T) {
	// ARRANGE
	original := BaseResponseMessage{Code: StatusCodeOK, Info: "ok", Data: "extra-payload"}

	// ACT
	encoded, err := original.MarshalBINN()
	require.NoError(t, err)

	var decoded []any
	require.NoError(t, binngo.Unmarshal(encoded, &decoded))

	// ASSERT
	require.Len(t, decoded, 3, "Data field must be appended after Code+Info when non-nil")
	assert.Equal(t, "extra-payload", decoded[2], "appended Data must be preserved verbatim")
}

func TestBaseResponseMessage_FillFromSlice_setsDataField_whenPresent(t *testing.T) {
	// ARRANGE
	var msg BaseResponseMessage

	// ACT
	err := msg.FillFromSlice([]any{uint8(100), "ok", "payload"})

	// ASSERT
	require.NoError(t, err)
	assert.Equal(t, StatusCodeOK, msg.Code)
	assert.Equal(t, "ok", msg.Info)
	assert.Equal(t, "payload", msg.Data, "third element must populate Data")
}

func TestLoginRequestMessage_UnmarshalBINN_extractsFields(t *testing.T) {
	// ARRANGE — slot 0 is the auth-mode marker, then login, password, sub-mode.
	raw := []any{ModeAuth, "alice", "secret", ModeCMD}
	encoded, err := binngo.Marshal(&raw)
	require.NoError(t, err)

	var msg LoginRequestMessage

	// ACT
	err = msg.UnmarshalBINN(encoded)

	// ASSERT
	require.NoError(t, err)
	assert.Equal(t, "alice", msg.Login, "login field must be populated from slot 1")
	assert.Equal(t, "secret", msg.Password, "password field must be populated from slot 2")
	assert.Equal(t, ModeCMD, msg.Mode, "mode field must be populated from slot 3")
}

func TestLoginRequestMessage_UnmarshalBINN_tooFewFields_returnsError(t *testing.T) {
	// ARRANGE
	raw := []any{ModeAuth, "alice", "secret"}
	encoded, err := binngo.Marshal(&raw)
	require.NoError(t, err)

	var msg LoginRequestMessage

	// ACT
	err = msg.UnmarshalBINN(encoded)

	// ASSERT
	require.Error(t, err)
	var invalid InvalidBINNValueError
	require.ErrorAs(t, err, &invalid, "must surface a typed InvalidBINNValueError")
	assert.Contains(t, err.Error(), "not enough values for LoginRequestMessage")
}

func TestLoginRequestMessage_UnmarshalBINN_loginNotString_returnsError(t *testing.T) {
	// ARRANGE — slot 1 must be a string per the protocol contract.
	raw := []any{ModeAuth, uint8(7), "secret", ModeCMD}
	encoded, err := binngo.Marshal(&raw)
	require.NoError(t, err)

	var msg LoginRequestMessage

	// ACT
	err = msg.UnmarshalBINN(encoded)

	// ASSERT
	require.Error(t, err)
	assert.Contains(t, err.Error(), "login is not a string")
}

func TestLoginRequestMessage_UnmarshalBINN_passwordNotString_returnsError(t *testing.T) {
	// ARRANGE
	raw := []any{ModeAuth, "alice", uint8(7), ModeCMD}
	encoded, err := binngo.Marshal(&raw)
	require.NoError(t, err)

	var msg LoginRequestMessage

	// ACT
	err = msg.UnmarshalBINN(encoded)

	// ASSERT
	require.Error(t, err)
	assert.Contains(t, err.Error(), "password is not a string")
}

func TestLoginRequestMessage_UnmarshalBINN_modeNotUint8_returnsError(t *testing.T) {
	// ARRANGE — using a string in the mode slot violates the wire contract.
	raw := []any{ModeAuth, "alice", "secret", "not-a-mode"}
	encoded, err := binngo.Marshal(&raw)
	require.NoError(t, err)

	var msg LoginRequestMessage

	// ACT
	err = msg.UnmarshalBINN(encoded)

	// ASSERT
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mode is not a uint8")
}

func TestInvalidBINNValueError_ErrorMessage_includesDetails(t *testing.T) {
	// ARRANGE
	err := NewInvalidBINNValueError("custom-detail")

	// ACT
	got := err.Error()

	// ASSERT
	assert.Equal(t, "invalid BINN value: custom-detail", got, "error string must surface the detail message verbatim")
}
