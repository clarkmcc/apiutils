package errors

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"net/http"
	"strconv"
)

// StatusError is an error intended for consumption by a REST API server; it can also be
// reconstructed by clients from a REST response. Public to allow easy type switches.
type StatusError struct {
	ErrStatus Status
}

// APIStatus is exposed by errors that can be converted to an api.Status object
// for finer grained details.
type APIStatus interface {
	Status() Status
}

var _ error = &StatusError{}

// Error implements the Error interface.
func (e *StatusError) Error() string {
	return e.ErrStatus.Message
}

// Status allows access to e's status without having to know the detailed workings
// of StatusError.
func (e *StatusError) Status() Status {
	return e.ErrStatus
}

// DebugError reports extended info about the error to debug output.
func (e *StatusError) DebugError() (string, []interface{}) {
	if out, err := json.MarshalIndent(e.ErrStatus, "", "  "); err == nil {
		return "server response object: %s", []interface{}{string(out)}
	}
	return "server response object: %#v", []interface{}{e.ErrStatus}
}

// HasStatusCause returns true if the provided error has a details cause
// with the provided type name.
func HasStatusCause(err error, name CauseType) bool {
	_, ok := GetStatusCause(err, name)
	return ok
}

// StatusCause returns the named cause from the provided error if it exists and
// the error is of the type APIStatus. Otherwise it returns false.
func GetStatusCause(err error, name CauseType) (StatusCause, bool) {
	apierr, ok := err.(APIStatus)
	if !ok || apierr == nil || apierr.Status().Details == nil {
		return StatusCause{}, false
	}
	for _, cause := range apierr.Status().Details.Causes {
		if cause.Type == name {
			return cause, true
		}
	}
	return StatusCause{}, false
}

// UnexpectedObjectError can be returned by FromObject if it's passed a non-status object.
type UnexpectedObjectError struct {
	Object interface{}
}

// Error returns an error message describing 'u'.
func (u *UnexpectedObjectError) Error() string {
	return fmt.Sprintf("unexpected object: %v", u.Object)
}

// FromObject generates an StatusError from an Status, if that is the type of obj; otherwise,
// returns an UnexpectedObjectError.
func FromObject(obj interface{}) error {
	switch t := obj.(type) {
	case *Status:
		return &StatusError{ErrStatus: *t}
	}
	return &UnexpectedObjectError{obj}
}

// NewNotFound returns a new error which indicates that the resource of the kind and the name was not found.
func NewNotFound(name string, uid string) *StatusError {
	var message string
	if len(uid) > 0 {
		message = fmt.Sprintf("%s (%s) not found", name, uid)
	} else {
		message = fmt.Sprintf("%s not found", name)
	}
	return &StatusError{Status{
		Status: StatusFailure,
		Code:   http.StatusNotFound,
		Reason: StatusReasonNotFound,
		Details: &StatusDetails{
			Name: name,
			UID:  uid,
		},
		Message: message,
	}}
}

// NewAlreadyExists returns an error indicating the item requested exists by that identifier.
func NewAlreadyExists(name string, uid string) *StatusError {
	var message string
	if len(uid) > 0 {
		message = fmt.Sprintf("%s (%s) not found", name, uid)
	} else {
		message = fmt.Sprintf("%s not found", name)
	}
	return &StatusError{Status{
		Status: StatusFailure,
		Code:   http.StatusConflict,
		Reason: StatusReasonAlreadyExists,
		Details: &StatusDetails{
			Name: name,
			UID:  uid,
		},
		Message: message,
	}}
}

// NewUnauthorized returns an error indicating the client is not authorized to perform the requested
// action.
func NewUnauthorized(reason string) *StatusError {
	message := reason
	if len(message) == 0 {
		message = "not authorized"
	}
	return &StatusError{Status{
		Status:  StatusFailure,
		Code:    http.StatusUnauthorized,
		Reason:  StatusReasonUnauthorized,
		Message: message,
	}}
}

// NewForbidden returns an error indicating the requested action was forbidden
func NewForbidden(name string, err error) *StatusError {
	message := fmt.Sprintf("forbidden: %v", err)
	return &StatusError{Status{
		Status: StatusFailure,
		Code:   http.StatusForbidden,
		Reason: StatusReasonForbidden,
		Details: &StatusDetails{
			Name: name,
		},
		Message: message,
	}}
}

// NewConflict returns an error indicating the item can't be updated as provided.
func NewConflict(name string, err error) *StatusError {
	return &StatusError{Status{
		Status: StatusFailure,
		Code:   http.StatusConflict,
		Reason: StatusReasonConflict,
		Details: &StatusDetails{
			Name: name,
		},
		Message: fmt.Sprintf("Operation cannot be fulfilled on %s: %v", name, err),
	}}
}

// NewInvalid returns an error indicating the item is invalid and cannot be processed.
func NewInvalid(name string, errs field.ErrorList) *StatusError {
	causes := make([]StatusCause, 0, len(errs))
	for i := range errs {
		err := errs[i]
		causes = append(causes, StatusCause{
			Type:    CauseType(err.Type),
			Message: err.ErrorBody(),
			Field:   err.Field,
		})
	}
	return &StatusError{Status{
		Status: StatusFailure,
		Code:   http.StatusUnprocessableEntity,
		Reason: StatusReasonInvalid,
		Details: &StatusDetails{
			Name:   name,
			Causes: causes,
		},
		Message: fmt.Sprintf("%s is invalid: %v", name, errs.ToAggregate()),
	}}
}

// NewBadRequest creates an error that indicates that the request is invalid and can not be processed.
func NewBadRequest(reason string) *StatusError {
	return &StatusError{Status{
		Status:  StatusFailure,
		Code:    http.StatusBadRequest,
		Reason:  StatusReasonBadRequest,
		Message: reason,
	}}
}

// NewTooManyRequests creates an error that indicates that the client must try again later because
// the specified endpoint is not accepting requests. More specific details should be provided
// if client should know why the failure was limited4.
func NewTooManyRequests(message string, retryAfterSeconds int) *StatusError {
	return &StatusError{Status{
		Status:  StatusFailure,
		Code:    http.StatusTooManyRequests,
		Reason:  StatusReasonTooManyRequests,
		Message: message,
		Details: &StatusDetails{
			RetryAfterSeconds: int32(retryAfterSeconds),
		},
	}}
}

// NewServiceUnavailable creates an error that indicates that the requested service is unavailable.
func NewServiceUnavailable(reason string) *StatusError {
	return &StatusError{Status{
		Status:  StatusFailure,
		Code:    http.StatusServiceUnavailable,
		Reason:  StatusReasonServiceUnavailable,
		Message: reason,
	}}
}

// NewMethodNotSupported returns an error indicating the requested action is not supported on this kind.
func NewMethodNotSupported(action string) *StatusError {
	return &StatusError{Status{
		Status:  StatusFailure,
		Code:    http.StatusMethodNotAllowed,
		Reason:  StatusReasonMethodNotAllowed,
		Message: fmt.Sprintf("%s is not supported on this resource", action),
	}}
}

// NewServerTimeout returns an error indicating the requested action could not be completed due to a
// transient error, and the client should try again.
func NewServerTimeout(operation string, retryAfterSeconds int) *StatusError {
	return &StatusError{Status{
		Status: StatusFailure,
		Code:   http.StatusInternalServerError,
		Reason: StatusReasonServerTimeout,
		Details: &StatusDetails{
			Name:              operation,
			RetryAfterSeconds: int32(retryAfterSeconds),
		},
		Message: fmt.Sprintf("The %s operation could not be completed at this time, please try again.", operation),
	}}
}

// NewInternalError returns an error indicating the item is invalid and cannot be processed.
func NewInternalError(err error) *StatusError {
	return &StatusError{Status{
		Status: StatusFailure,
		Code:   http.StatusInternalServerError,
		Reason: StatusReasonInternalError,
		Details: &StatusDetails{
			Causes: []StatusCause{{Message: err.Error()}},
		},
		Message: fmt.Sprintf("Internal error occurred: %v", err),
	}}
}

// NewTimeoutError returns an error indicating that a timeout occurred before the request
// could be completed.  Clients may retry, but the operation may still complete.
func NewTimeoutError(message string, retryAfterSeconds int) *StatusError {
	return &StatusError{Status{
		Status:  StatusFailure,
		Code:    http.StatusGatewayTimeout,
		Reason:  StatusReasonTimeout,
		Message: fmt.Sprintf("Timeout: %s", message),
		Details: &StatusDetails{
			RetryAfterSeconds: int32(retryAfterSeconds),
		},
	}}
}

// NewTooManyRequestsError returns an error indicating that the request was rejected because
// the server has received too many requests. Client should wait and retry. But if the request
// is perishable, then the client should not retry the request.
func NewTooManyRequestsError(message string) *StatusError {
	return &StatusError{Status{
		Status:  StatusFailure,
		Code:    http.StatusTooManyRequests,
		Reason:  StatusReasonTooManyRequests,
		Message: fmt.Sprintf("Too many requests: %s", message),
	}}
}

// NewRequestEntityTooLargeError returns an error indicating that the request
// entity was too large.
func NewRequestEntityTooLargeError(message string) *StatusError {
	return &StatusError{Status{
		Status:  StatusFailure,
		Code:    http.StatusRequestEntityTooLarge,
		Reason:  StatusReasonRequestEntityTooLarge,
		Message: fmt.Sprintf("Request entity too large: %s", message),
	}}
}

// NewGenericServerResponse returns a new error for server responses that are not in a recognizable form.
func NewGenericServerResponse(code int, verb string, name, serverMessage string, retryAfterSeconds int, isUnexpectedResponse bool) *StatusError {
	reason := StatusReasonUnknown
	message := fmt.Sprintf("the server responded with the status code %d but did not return more information", code)
	switch code {
	case http.StatusConflict:
		if verb == "POST" {
			reason = StatusReasonAlreadyExists
		} else {
			reason = StatusReasonConflict
		}
		message = "the server reported a conflict"
	case http.StatusNotFound:
		reason = StatusReasonNotFound
		message = "the server could not find the requested resource"
	case http.StatusBadRequest:
		reason = StatusReasonBadRequest
		message = "the server rejected our request for an unknown reason"
	case http.StatusUnauthorized:
		reason = StatusReasonUnauthorized
		message = "the server has asked for the client to provide credentials"
	case http.StatusForbidden:
		reason = StatusReasonForbidden
		// the server message has details about who is trying to perform what action.  Keep its message.
		message = serverMessage
	case http.StatusNotAcceptable:
		reason = StatusReasonNotAcceptable
		// the server message has details about what types are acceptable
		if len(serverMessage) == 0 || serverMessage == "unknown" {
			message = "the server was unable to respond with a content type that the client supports"
		} else {
			message = serverMessage
		}
	case http.StatusUnsupportedMediaType:
		reason = StatusReasonUnsupportedMediaType
		// the server message has details about what types are acceptable
		message = serverMessage
	case http.StatusMethodNotAllowed:
		reason = StatusReasonMethodNotAllowed
		message = "the server does not allow this method on the requested resource"
	case http.StatusUnprocessableEntity:
		reason = StatusReasonInvalid
		message = "the server rejected our request due to an error in our request"
	case http.StatusServiceUnavailable:
		reason = StatusReasonServiceUnavailable
		message = "the server is currently unable to handle the request"
	case http.StatusGatewayTimeout:
		reason = StatusReasonTimeout
		message = "the server was unable to return a response in the time allotted, but may still be processing the request"
	case http.StatusTooManyRequests:
		reason = StatusReasonTooManyRequests
		message = "the server has received too many requests and has asked us to try again later"
	default:
		if code >= 500 {
			reason = StatusReasonInternalError
			message = fmt.Sprintf("an error on the server (%q) has prevented the request from succeeding", serverMessage)
		}
	}
	var causes []StatusCause
	if isUnexpectedResponse {
		causes = []StatusCause{
			{
				Type:    CauseTypeUnexpectedServerResponse,
				Message: serverMessage,
			},
		}
	} else {
		causes = nil
	}
	return &StatusError{Status{
		Status: StatusFailure,
		Code:   int32(code),
		Reason: reason,
		Details: &StatusDetails{
			Name:              name,
			Causes:            causes,
			RetryAfterSeconds: int32(retryAfterSeconds),
		},
		Message: message,
	}}
}

// IsNotFound returns true if the specified error was created by NewNotFound.
// It supports wrapped errors.
func IsNotFound(err error) bool {
	return ReasonForError(err) == StatusReasonNotFound
}

// IsAlreadyExists determines if the err is an error which indicates that a specified resource already exists.
// It supports wrapped errors.
func IsAlreadyExists(err error) bool {
	return ReasonForError(err) == StatusReasonAlreadyExists
}

// IsConflict determines if the err is an error which indicates the provided update conflicts.
// It supports wrapped errors.
func IsConflict(err error) bool {
	return ReasonForError(err) == StatusReasonConflict
}

// IsInvalid determines if the err is an error which indicates the provided resource is not valid.
// It supports wrapped errors.
func IsInvalid(err error) bool {
	return ReasonForError(err) == StatusReasonInvalid
}

// IsNotAcceptable determines if err is an error which indicates that the request failed due to an invalid Accept header
// It supports wrapped errors.
func IsNotAcceptable(err error) bool {
	return ReasonForError(err) == StatusReasonNotAcceptable
}

// IsUnsupportedMediaType determines if err is an error which indicates that the request failed due to an invalid Content-Type header
// It supports wrapped errors.
func IsUnsupportedMediaType(err error) bool {
	return ReasonForError(err) == StatusReasonUnsupportedMediaType
}

// IsMethodNotSupported determines if the err is an error which indicates the provided action could not
// be performed because it is not supported by the server.
// It supports wrapped errors.
func IsMethodNotSupported(err error) bool {
	return ReasonForError(err) == StatusReasonMethodNotAllowed
}

// IsServiceUnavailable is true if the error indicates the underlying service is no longer available.
// It supports wrapped errors.
func IsServiceUnavailable(err error) bool {
	return ReasonForError(err) == StatusReasonServiceUnavailable
}

// IsBadRequest determines if err is an error which indicates that the request is invalid.
// It supports wrapped errors.
func IsBadRequest(err error) bool {
	return ReasonForError(err) == StatusReasonBadRequest
}

// IsUnauthorized determines if err is an error which indicates that the request is unauthorized and
// requires authentication by the user.
// It supports wrapped errors.
func IsUnauthorized(err error) bool {
	return ReasonForError(err) == StatusReasonUnauthorized
}

// IsForbidden determines if err is an error which indicates that the request is forbidden and cannot
// be completed as requested.
// It supports wrapped errors.
func IsForbidden(err error) bool {
	return ReasonForError(err) == StatusReasonForbidden
}

// IsTimeout determines if err is an error which indicates that request times out due to long
// processing.
// It supports wrapped errors.
func IsTimeout(err error) bool {
	return ReasonForError(err) == StatusReasonTimeout
}

// IsServerTimeout determines if err is an error which indicates that the request needs to be retried
// by the client.
// It supports wrapped errors.
func IsServerTimeout(err error) bool {
	return ReasonForError(err) == StatusReasonServerTimeout
}

// IsInternalError determines if err is an error which indicates an internal server error.
// It supports wrapped errors.
func IsInternalError(err error) bool {
	return ReasonForError(err) == StatusReasonInternalError
}

// IsTooManyRequests determines if err is an error which indicates that there are too many requests
// that the server cannot handle.
// It supports wrapped errors.
func IsTooManyRequests(err error) bool {
	if ReasonForError(err) == StatusReasonTooManyRequests {
		return true
	}
	if status := APIStatus(nil); errors.As(err, &status) {
		return status.Status().Code == http.StatusTooManyRequests
	}
	return false
}

// IsRequestEntityTooLargeError determines if err is an error which indicates
// the request entity is too large.
// It supports wrapped errors.
func IsRequestEntityTooLargeError(err error) bool {
	if ReasonForError(err) == StatusReasonRequestEntityTooLarge {
		return true
	}
	if status := APIStatus(nil); errors.As(err, &status) {
		return status.Status().Code == http.StatusRequestEntityTooLarge
	}
	return false
}

// IsUnexpectedServerError returns true if the server response was not in the expected API format,
// and may be the result of another HTTP actor.
// It supports wrapped errors.
func IsUnexpectedServerError(err error) bool {
	if status := APIStatus(nil); errors.As(err, &status) && status.Status().Details != nil {
		for _, cause := range status.Status().Details.Causes {
			if cause.Type == CauseTypeUnexpectedServerResponse {
				return true
			}
		}
	}
	return false
}

// IsUnexpectedObjectError determines if err is due to an unexpected object from the master.
// It supports wrapped errors.
func IsUnexpectedObjectError(err error) bool {
	uoe := &UnexpectedObjectError{}
	return err != nil && errors.As(err, &uoe)
}

// SuggestsClientDelay returns true if this error suggests a client delay as well as the
// suggested seconds to wait, or false if the error does not imply a wait. It does not
// address whether the error *should* be retried, since some errors (like a 3xx) may
// request delay without retry.
// It supports wrapped errors.
func SuggestsClientDelay(err error) (int, bool) {
	if t := APIStatus(nil); errors.As(err, &t) && t.Status().Details != nil {
		switch t.Status().Reason {
		// this StatusReason explicitly requests the caller to delay the action
		case StatusReasonServerTimeout:
			return int(t.Status().Details.RetryAfterSeconds), true
		}
		// If the client requests that we retry after a certain number of seconds
		if t.Status().Details.RetryAfterSeconds > 0 {
			return int(t.Status().Details.RetryAfterSeconds), true
		}
	}
	return 0, false
}

// ReasonForError returns the HTTP status for a particular error.
// It supports wrapped errors.
func ReasonForError(err error) StatusReason {
	if status := APIStatus(nil); errors.As(err, &status) {
		return status.Status().Reason
	}
	return StatusReasonUnknown
}

// ErrorToAPIStatus converts an error to an Status object.
func ErrorToAPIStatus(err error) *Status {
	switch t := err.(type) {
	case interface{ Status() Status }:
		status := t.Status()
		if len(status.Status) == 0 {
			status.Status = StatusFailure
		}
		switch status.Status {
		case StatusSuccess:
			if status.Code == 0 {
				status.Code = http.StatusOK
			}
		case StatusFailure:
			if status.Code == 0 {
				status.Code = http.StatusInternalServerError
			}
		default:
			runtime.HandleError(fmt.Errorf("apiserver received an error with wrong status field : %#+v", err))
			if status.Code == 0 {
				status.Code = http.StatusInternalServerError
			}
		}
		return &status
	default:
		status := http.StatusInternalServerError
		// Log errors that were not converted to an error status
		// by REST storage - these typically indicate programmer
		// error by not using pkg/api/errors, or unexpected failure
		// cases.
		return &Status{
			Status:  StatusFailure,
			Code:    int32(status),
			Reason:  StatusReasonUnknown,
			Message: err.Error(),
		}
	}
}

// FromResponse determines if the http.Response contains an error, if so, it
// attempts to decode the error into a Status struct. If the decoding fails, an
// internal error is returned
func FromResponse(resp *http.Response) (err error, hasError bool) {
	if resp.StatusCode >= http.StatusOK && resp.StatusCode <= http.StatusNoContent {
		return nil, false
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return NewInternalError(fmt.Errorf("client error: reading server response: %w", err)), true
	}
	status := Status{}
	err = json.Unmarshal(body, &status)
	if err != nil {
		return NewInternalError(fmt.Errorf("client error: unmarshalling server response: %w", err)), true
	}
	seconds, ok := retryAfterSeconds(resp)
	if ok {
		if status.Details == nil {
			status.Details = &StatusDetails{
				RetryAfterSeconds: int32(seconds),
			}
		} else {
			status.Details.RetryAfterSeconds = int32(seconds)
		}
	}
	return &StatusError{ErrStatus: status}, true
}

// retryAfterSeconds returns the value of the Retry-After header and true, or 0 and false if
// the header was missing or not a valid number.
func retryAfterSeconds(resp *http.Response) (int, bool) {
	if h := resp.Header.Get("Retry-After"); len(h) > 0 {
		if i, err := strconv.Atoi(h); err == nil {
			return i, true
		}
	}
	return 0, false
}

// checkWait returns true along with a number of seconds if the server instructed us to wait
// before retrying.
func checkWait(resp *http.Response) (int, bool) {
	switch r := resp.StatusCode; {
	// any 500 error code and 429 can trigger a wait
	case r == http.StatusTooManyRequests, r >= 500:
	default:
		return 0, false
	}
	i, ok := retryAfterSeconds(resp)
	return i, ok
}
