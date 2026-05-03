package archiver

import (
	"bytes"
	"compress/flate"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeflateCompressorFor_RoundTrip(t *testing.T) {
	t.Parallel()

	payload := bytes.Repeat([]byte("Hello, World! "), 200)

	tests := []struct {
		name    string
		inLevel int
	}{
		{name: "negative_clamps_to_default", inLevel: -5},
		{name: "zero_below_best_speed_clamps_to_default", inLevel: 0},
		{name: "best_speed_passthrough", inLevel: flate.BestSpeed},
		{name: "mid_value_passthrough", inLevel: 5},
		{name: "best_compression_passthrough", inLevel: flate.BestCompression},
		{name: "over_max_clamps_to_best_compression", inLevel: flate.BestCompression + 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// ARRANGE
			factory := deflateCompressorFor(tt.inLevel)

			// ACT
			var buf bytes.Buffer
			w, err := factory(&buf)
			require.NoError(t, err)
			_, err = w.Write(payload)
			require.NoError(t, err)
			require.NoError(t, w.Close())

			r := flate.NewReader(&buf)
			got, err := io.ReadAll(r)
			require.NoError(t, err)
			require.NoError(t, r.Close())

			// ASSERT
			assert.Equal(t, payload, got, "round-trip must yield the original payload")
		})
	}
}

func TestDeflateCompressorFor_ReturnsIndependentWriters(t *testing.T) {
	t.Parallel()

	// ARRANGE
	factory := deflateCompressorFor(flate.BestSpeed)
	payloadA := []byte("alpha-content-stream-AAAA")
	payloadB := []byte("bravo-content-stream-BBBB")

	// ACT
	var bufA, bufB bytes.Buffer
	wA, err := factory(&bufA)
	require.NoError(t, err)
	wB, err := factory(&bufB)
	require.NoError(t, err)

	_, err = wA.Write(payloadA)
	require.NoError(t, err)
	_, err = wB.Write(payloadB)
	require.NoError(t, err)
	require.NoError(t, wA.Close())
	require.NoError(t, wB.Close())

	rA := flate.NewReader(&bufA)
	gotA, err := io.ReadAll(rA)
	require.NoError(t, err)
	require.NoError(t, rA.Close())

	rB := flate.NewReader(&bufB)
	gotB, err := io.ReadAll(rB)
	require.NoError(t, err)
	require.NoError(t, rB.Close())

	// ASSERT
	assert.Equal(t, payloadA, gotA, "writer A must be independent")
	assert.Equal(t, payloadB, gotB, "writer B must be independent")
}
