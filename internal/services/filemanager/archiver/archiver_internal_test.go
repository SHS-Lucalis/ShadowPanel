package archiver

import (
	"bytes"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSafeUnix(t *testing.T) {
	t.Parallel()

	t.Run("zero_returns_now", func(t *testing.T) {
		t.Parallel()

		// ARRANGE
		before := time.Now().Unix()

		// ACT
		got := safeUnix(0)

		// ASSERT
		after := time.Now().Unix()
		assert.GreaterOrEqual(t, got, before-2, "got must not be before captured 'before' (with tolerance)")
		assert.LessOrEqual(t, got, after+2, "got must not be after captured 'after' (with tolerance)")
	})

	t.Run("over_max_returns_now", func(t *testing.T) {
		t.Parallel()

		// ARRANGE
		before := time.Now().Unix()

		// ACT
		got := safeUnix((1 << 62) + 1)

		// ASSERT
		after := time.Now().Unix()
		assert.GreaterOrEqual(t, got, before-2)
		assert.LessOrEqual(t, got, after+2)
	})

	t.Run("normal_returns_input", func(t *testing.T) {
		t.Parallel()

		// ACT
		got := safeUnix(1700000000)

		// ASSERT
		assert.Equal(t, int64(1700000000), got)
	})
}

type errWriter struct {
	err error
}

func (e *errWriter) Write(_ []byte) (int, error) {
	return 0, e.err
}

func TestCountingWriter(t *testing.T) {
	t.Parallel()

	t.Run("tracks_bytes_and_total", func(t *testing.T) {
		t.Parallel()

		// ARRANGE
		var buf bytes.Buffer
		cw := &countingWriter{w: &buf}

		// ACT
		n1, err := cw.Write([]byte("ab"))
		require.NoError(t, err)
		n2, err := cw.Write([]byte("cd"))
		require.NoError(t, err)

		// ASSERT
		assert.Equal(t, 2, n1)
		assert.Equal(t, 2, n2)
		assert.Equal(t, 4, cw.bytesWritten, "interim bytesWritten must match total written")
		assert.Equal(t, uint64(4), cw.totalBytesWritten, "totalBytesWritten must match total written")
		assert.Equal(t, "abcd", buf.String())
	})

	t.Run("propagates_underlying_error", func(t *testing.T) {
		t.Parallel()

		// ARRANGE
		boom := errors.New("write boom")
		cw := &countingWriter{w: &errWriter{err: boom}}

		// ACT
		n, err := cw.Write([]byte("payload"))

		// ASSERT
		require.Error(t, err)
		assert.Contains(t, err.Error(), "write boom")
		assert.Equal(t, 0, n, "errWriter reports zero bytes written")
		assert.Equal(t, 0, cw.bytesWritten, "no bytes written must mean counter unchanged")
		assert.Equal(t, uint64(0), cw.totalBytesWritten)
	})
}

func TestFlusherFor(t *testing.T) {
	t.Parallel()

	t.Run("nil_for_plain_buffer", func(t *testing.T) {
		t.Parallel()

		// ARRANGE
		var buf bytes.Buffer

		// ACT
		fn := flusherFor(&buf)

		// ASSERT
		assert.Nil(t, fn, "plain buffer is neither ResponseWriter nor Flusher")
	})

	t.Run("non_nil_for_responsewriter", func(t *testing.T) {
		t.Parallel()

		// ARRANGE
		rec := httptest.NewRecorder()

		// ACT
		fn := flusherFor(rec)

		// ASSERT
		require.NotNil(t, fn, "ResponseWriter must produce a flusher")
		assert.NotPanics(t, fn)
	})
}
