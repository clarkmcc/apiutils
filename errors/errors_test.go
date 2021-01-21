package errors

import (
	"errors"
	"fmt"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"net/http"
	"reflect"
	"testing"
)

func TestErrorNew(t *testing.T) {
	err := NewAlreadyExists("tests", "1")
	if !IsAlreadyExists(err) {
		t.Errorf("expected to be %s", StatusReasonAlreadyExists)
	}
	if IsConflict(err) {
		t.Errorf("expected to not be %s", StatusReasonConflict)
	}
	if IsNotFound(err) {
		t.Errorf(fmt.Sprintf("expected to not be %s", StatusReasonNotFound))
	}
	if IsInvalid(err) {
		t.Errorf("expected to not be %s", StatusReasonInvalid)
	}
	if IsBadRequest(err) {
		t.Errorf("expected to not be %s", StatusReasonBadRequest)
	}
	if IsForbidden(err) {
		t.Errorf("expected to not be %s", StatusReasonForbidden)
	}
	if IsServerTimeout(err) {
		t.Errorf("expected to not be %s", StatusReasonServerTimeout)
	}
	if IsMethodNotSupported(err) {
		t.Errorf("expected to not be %s", StatusReasonMethodNotAllowed)
	}

	if !IsConflict(NewConflict("tests", errors.New("message"))) {
		t.Errorf("expected to be conflict")
	}
	if !IsNotFound(NewNotFound("tests", "3")) {
		t.Errorf("expected to be %s", StatusReasonNotFound)
	}
	if !IsInvalid(NewInvalid("tests", nil)) {
		t.Errorf("expected to be %s", StatusReasonInvalid)
	}
	if !IsBadRequest(NewBadRequest("reason")) {
		t.Errorf("expected to be %s", StatusReasonBadRequest)
	}
	if !IsForbidden(NewForbidden("tests", errors.New("reason"))) {
		t.Errorf("expected to be %s", StatusReasonForbidden)
	}
	if !IsUnauthorized(NewUnauthorized("reason")) {
		t.Errorf("expected to be %s", StatusReasonUnauthorized)
	}
	if !IsServerTimeout(NewServerTimeout("tests", 0)) {
		t.Errorf("expected to be %s", StatusReasonServerTimeout)
	}
	if !IsMethodNotSupported(NewMethodNotSupported("delete")) {
		t.Errorf("expected to be %s", StatusReasonMethodNotAllowed)
	}

	if time, ok := SuggestsClientDelay(NewServerTimeout("doing something", 10)); time != 10 || !ok {
		t.Errorf("unexpected %d", time)
	}
	if time, ok := SuggestsClientDelay(NewServerTimeout("doing something", 0)); time != 0 || !ok {
		t.Errorf("unexpected %d", time)
	}
	if time, ok := SuggestsClientDelay(NewTimeoutError("test reason", 10)); time != 10 || !ok {
		t.Errorf("unexpected %d", time)
	}
	if time, ok := SuggestsClientDelay(NewTooManyRequests("doing something", 10)); time != 10 || !ok {
		t.Errorf("unexpected %d", time)
	}
	if time, ok := SuggestsClientDelay(NewTooManyRequests("doing something", 1)); time != 1 || !ok {
		t.Errorf("unexpected %d", time)
	}
	if time, ok := SuggestsClientDelay(NewGenericServerResponse(429, "get", "tests", "doing something", 10, true)); time != 10 || !ok {
		t.Errorf("unexpected %d", time)
	}
	if time, ok := SuggestsClientDelay(NewGenericServerResponse(500, "get", "tests", "doing something", 10, true)); time != 10 || !ok {
		t.Errorf("unexpected %d", time)
	}
	if time, ok := SuggestsClientDelay(NewGenericServerResponse(429, "get", "tests", "doing something", 0, true)); time != 0 || ok {
		t.Errorf("unexpected %d", time)
	}
}

func TestNewInvalid(t *testing.T) {
	testCases := []struct {
		Err     *field.Error
		Details *StatusDetails
	}{
		{
			field.Duplicate(field.NewPath("field[0].name"), "bar"),
			&StatusDetails{
				Name: "name",
				Causes: []StatusCause{{
					Type:  CauseTypeFieldValueDuplicate,
					Field: "field[0].name",
				}},
			},
		},
		{
			field.Invalid(field.NewPath("field[0].name"), "bar", "detail"),
			&StatusDetails{
				Name: "name",
				Causes: []StatusCause{{
					Type:  CauseTypeFieldValueInvalid,
					Field: "field[0].name",
				}},
			},
		},
		{
			field.NotFound(field.NewPath("field[0].name"), "bar"),
			&StatusDetails{
				Name: "name",
				Causes: []StatusCause{{
					Type:  CauseTypeFieldValueNotFound,
					Field: "field[0].name",
				}},
			},
		},
		{
			field.NotSupported(field.NewPath("field[0].name"), "bar", nil),
			&StatusDetails{
				Name: "name",
				Causes: []StatusCause{{
					Type:  CauseTypeFieldValueNotSupported,
					Field: "field[0].name",
				}},
			},
		},
		{
			field.Required(field.NewPath("field[0].name"), ""),
			&StatusDetails{
				Name: "name",
				Causes: []StatusCause{{
					Type:  CauseTypeFieldValueRequired,
					Field: "field[0].name",
				}},
			},
		},
	}
	for i, testCase := range testCases {
		vErr, expected := testCase.Err, testCase.Details
		expected.Causes[0].Message = vErr.ErrorBody()
		err := NewInvalid("name", field.ErrorList{vErr})
		status := err.ErrStatus
		if status.Code != 422 || status.Reason != StatusReasonInvalid {
			t.Errorf("%d: unexpected status: %#v", i, status)
		}
		if !reflect.DeepEqual(expected, status.Details) {
			t.Errorf("%d: expected %#v, got %#v", i, expected, status.Details)
		}
	}
}

func TestReasonForError(t *testing.T) {
	if e, a := StatusReasonUnknown, ReasonForError(nil); e != a {
		t.Errorf("unexpected reason type: %#v", a)
	}
}

type TestType struct{}

func TestFromObject(t *testing.T) {
	table := []struct {
		obj     interface{}
		message string
	}{
		{&Status{Message: "foobar"}, "foobar"},
		{&TestType{}, "unexpected object: &{}"},
	}

	for _, item := range table {
		if e, a := item.message, FromObject(item.obj).Error(); e != a {
			t.Errorf("Expected %v, got %v", e, a)
		}
	}
}

func TestReasonForErrorSupportsWrappedErrors(t *testing.T) {
	testCases := []struct {
		name           string
		err            error
		expectedReason StatusReason
	}{
		{
			name:           "Direct match",
			err:            &StatusError{ErrStatus: Status{Reason: StatusReasonUnauthorized}},
			expectedReason: StatusReasonUnauthorized,
		},
		{
			name:           "No match",
			err:            errors.New("some other error"),
			expectedReason: StatusReasonUnknown,
		},
		{
			name:           "Nested match",
			err:            fmt.Errorf("wrapping: %w", fmt.Errorf("some more: %w", &StatusError{ErrStatus: Status{Reason: StatusReasonAlreadyExists}})),
			expectedReason: StatusReasonAlreadyExists,
		},
		{
			name:           "Nested, no match",
			err:            fmt.Errorf("wrapping: %w", fmt.Errorf("some more: %w", errors.New("hello"))),
			expectedReason: StatusReasonUnknown,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if result := ReasonForError(tc.err); result != tc.expectedReason {
				t.Errorf("expected reason: %q, but got known reason: %q", tc.expectedReason, result)
			}
		})
	}
}

func TestIsTooManyRequestsSupportsWrappedErrors(t *testing.T) {
	testCases := []struct {
		name        string
		err         error
		expectMatch bool
	}{
		{
			name:        "Direct match via status reason",
			err:         &StatusError{ErrStatus: Status{Reason: StatusReasonTooManyRequests}},
			expectMatch: true,
		},
		{
			name:        "Direct match via status code",
			err:         &StatusError{ErrStatus: Status{Code: http.StatusTooManyRequests}},
			expectMatch: true,
		},
		{
			name:        "No match",
			err:         &StatusError{},
			expectMatch: false,
		},
		{
			name:        "Nested match via status reason",
			err:         fmt.Errorf("Wrapping: %w", &StatusError{ErrStatus: Status{Reason: StatusReasonTooManyRequests}}),
			expectMatch: true,
		},
		{
			name:        "Nested match via status code",
			err:         fmt.Errorf("Wrapping: %w", &StatusError{ErrStatus: Status{Code: http.StatusTooManyRequests}}),
			expectMatch: true,
		},
		{
			name:        "Nested,no match",
			err:         fmt.Errorf("Wrapping: %w", &StatusError{ErrStatus: Status{Code: http.StatusNotFound}}),
			expectMatch: false,
		},
	}

	for _, tc := range testCases {
		if result := IsTooManyRequests(tc.err); result != tc.expectMatch {
			t.Errorf("Expect match %t, got match %t", tc.expectMatch, result)
		}
	}
}
func TestIsRequestEntityTooLargeErrorSupportsWrappedErrors(t *testing.T) {
	testCases := []struct {
		name        string
		err         error
		expectMatch bool
	}{
		{
			name:        "Direct match via status reason",
			err:         &StatusError{ErrStatus: Status{Reason: StatusReasonRequestEntityTooLarge}},
			expectMatch: true,
		},
		{
			name:        "Direct match via status code",
			err:         &StatusError{ErrStatus: Status{Code: http.StatusRequestEntityTooLarge}},
			expectMatch: true,
		},
		{
			name:        "No match",
			err:         &StatusError{},
			expectMatch: false,
		},
		{
			name:        "Nested match via status reason",
			err:         fmt.Errorf("Wrapping: %w", &StatusError{ErrStatus: Status{Reason: StatusReasonRequestEntityTooLarge}}),
			expectMatch: true,
		},
		{
			name:        "Nested match via status code",
			err:         fmt.Errorf("Wrapping: %w", &StatusError{ErrStatus: Status{Code: http.StatusRequestEntityTooLarge}}),
			expectMatch: true,
		},
		{
			name:        "Nested,no match",
			err:         fmt.Errorf("Wrapping: %w", &StatusError{ErrStatus: Status{Code: http.StatusNotFound}}),
			expectMatch: false,
		},
	}

	for _, tc := range testCases {
		if result := IsRequestEntityTooLargeError(tc.err); result != tc.expectMatch {
			t.Errorf("Expect match %t, got match %t", tc.expectMatch, result)
		}
	}
}

func TestIsUnexpectedServerError(t *testing.T) {
	unexpectedServerErr := func() error {
		return &StatusError{
			ErrStatus: Status{
				Details: &StatusDetails{
					Causes: []StatusCause{{Type: CauseTypeUnexpectedServerResponse}},
				},
			},
		}
	}
	testCases := []struct {
		name        string
		err         error
		expectMatch bool
	}{
		{
			name:        "Direct match",
			err:         unexpectedServerErr(),
			expectMatch: true,
		},
		{
			name:        "No match",
			err:         errors.New("some other error"),
			expectMatch: false,
		},
		{
			name:        "Nested match",
			err:         fmt.Errorf("wrapping: %w", unexpectedServerErr()),
			expectMatch: true,
		},
		{
			name:        "Nested, no match",
			err:         fmt.Errorf("wrapping: %w", fmt.Errorf("some more: %w", errors.New("hello"))),
			expectMatch: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if result := IsUnexpectedServerError(tc.err); result != tc.expectMatch {
				t.Errorf("expected match: %t, but got match: %t", tc.expectMatch, result)
			}
		})
	}
}

func TestIsUnexpectedObjectError(t *testing.T) {
	unexpectedObjectErr := func() error {
		return &UnexpectedObjectError{}
	}
	testCases := []struct {
		name        string
		err         error
		expectMatch bool
	}{
		{
			name:        "Direct match",
			err:         unexpectedObjectErr(),
			expectMatch: true,
		},
		{
			name:        "No match",
			err:         errors.New("some other error"),
			expectMatch: false,
		},
		{
			name:        "Nested match",
			err:         fmt.Errorf("wrapping: %w", unexpectedObjectErr()),
			expectMatch: true,
		},
		{
			name:        "Nested, no match",
			err:         fmt.Errorf("wrapping: %w", fmt.Errorf("some more: %w", errors.New("hello"))),
			expectMatch: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if result := IsUnexpectedObjectError(tc.err); result != tc.expectMatch {
				t.Errorf("expected match: %t, but got match: %t", tc.expectMatch, result)
			}
		})
	}
}

func TestSuggestsClientDelaySupportsWrapping(t *testing.T) {
	suggestsClientDelayErr := func() error {
		return &StatusError{
			ErrStatus: Status{
				Reason:  StatusReasonServerTimeout,
				Details: &StatusDetails{},
			},
		}
	}
	testCases := []struct {
		name        string
		err         error
		expectMatch bool
	}{
		{
			name:        "Direct match",
			err:         suggestsClientDelayErr(),
			expectMatch: true,
		},
		{
			name:        "No match",
			err:         errors.New("some other error"),
			expectMatch: false,
		},
		{
			name:        "Nested match",
			err:         fmt.Errorf("wrapping: %w", suggestsClientDelayErr()),
			expectMatch: true,
		},
		{
			name:        "Nested, no match",
			err:         fmt.Errorf("wrapping: %w", fmt.Errorf("some more: %w", errors.New("hello"))),
			expectMatch: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if _, result := SuggestsClientDelay(tc.err); result != tc.expectMatch {
				t.Errorf("expected match: %t, but got match: %t", tc.expectMatch, result)
			}
		})
	}
}
