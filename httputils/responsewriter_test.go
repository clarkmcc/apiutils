package httputils

import (
	"github.com/clarkmcc/apiutils/errors"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		WriteError(errors.NewNotFound("test", ""), w)
	}))
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	require.NoError(t, err)

	err, hasError := errors.FromResponse(resp)
	require.True(t, hasError)
	require.True(t, errors.IsNotFound(err))
}
