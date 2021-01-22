package httputils

import (
	"encoding/json"
	"github.com/clarkmcc/apiutils/errors"
	"net/http"
	"strconv"
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
func WriteError(err error, w http.ResponseWriter) {
	status := errors.ErrorToAPIStatus(err)
	// when writing an error, check to see if the status indicates a retry after period
	if status.Details != nil && status.Details.RetryAfterSeconds > 0 {
		delay := strconv.Itoa(int(status.Details.RetryAfterSeconds))
		w.Header().Set("Retry-After", delay)
	}
	WriteRawJSON(int(status.Code), status, w)
}
