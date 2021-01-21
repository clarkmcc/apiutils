package http

import (
	"encoding/json"
	"github.com/clarkmcc/apiutils/errors"
	"net/http"
)

// WriteRawJSON writes a non-API object in JSON.
func WriteRawJSON(statusCode int, object interface{}, w http.ResponseWriter) {
	output, err := json.MarshalIndent(object, "", "  ")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	w.Write(output)
}

// WriteError wraps WriteRawJSON and writes the appropriate error to the response writer
func WriteError(err *errors.StatusError, w http.ResponseWriter) {
	WriteRawJSON(int(err.ErrStatus.Code), err.ErrStatus, w)
}
