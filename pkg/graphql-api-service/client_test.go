package graphqlapiservice

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

type testSuite struct {
	api     *Client
	httpCli *MockhttpClient
	ctrl    *gomock.Controller
}

func newMockClient(t *testing.T) testSuite {
	ctrl := gomock.NewController(t)
	cli := NewMockhttpClient(ctrl)
	return testSuite{
		api: &Client{
			cli:          cli,
			problemCache: newCache(),
		},
		httpCli: cli,
		ctrl:    ctrl,
	}
}

func TestUnit_DoRequest(t *testing.T) {
	s := newMockClient(t)
	defer s.ctrl.Finish()

	testCases := map[string]struct {
		mock   func()
		output []byte
		err    error
	}{
		"normal request": {
			mock: func() {
				jsonResponse := "{\"data\":123}"
				recorder := httptest.NewRecorder()
				recorder.Body.WriteString(jsonResponse)
				response := recorder.Result() //nolint:bodyclose

				s.httpCli.EXPECT().Do(gomock.Any()).Return(response, nil)
			},
			output: []byte("123"),
			err:    nil,
		},
		"http error": {
			mock: func() {
				s.httpCli.EXPECT().Do(gomock.Any()).Return(nil, fmt.Errorf("test error"))
			},
			output: nil,
			err:    fmt.Errorf("test error"),
		},
	}

	for name, test := range testCases {
		t.Run(name, func(t *testing.T) {
			if test.mock != nil {
				test.mock()
			}
			body, err := s.api.doRequest(&http.Request{})
			assert.Equal(t, test.output, body)
			if test.err != nil {
				assert.Error(t, err)
			}
		})
	}
}

func TestUnit_RefreshCSRFToken(t *testing.T) {
	s := newMockClient(t)
	defer s.ctrl.Finish()

	testCases := map[string]struct {
		mock      func()
		csrfToken string
		err       error
	}{
		"normal request": {
			mock: func() {
				recorder := httptest.NewRecorder()
				http.SetCookie(recorder, &http.Cookie{Name: csrfTokenCookie, Value: "expected_cookie"})
				response := recorder.Result()
				err := response.Body.Close()
				assert.NoError(t, err)
				s.httpCli.EXPECT().Do(gomock.Any()).Return(response, nil)
			},
			csrfToken: "expected_cookie",
			err:       nil,
		},
		"csrf not expired": {
			mock: func() {
				s.api.csrf = &http.Cookie{Name: csrfTokenCookie, Value: "expected_cookie", Expires: time.Now().Add(1 * time.Hour)}
				// no call expected
			},
			csrfToken: "expected_cookie",
			err:       nil,
		},
		"http error": {
			mock: func() {
				s.api.csrf = nil
				s.httpCli.EXPECT().Do(gomock.Any()).Return(nil, fmt.Errorf("test error"))
			},
			csrfToken: "expected_cookie",
			err:       fmt.Errorf("test error"),
		},
	}

	for name, test := range testCases {
		t.Run(name, func(t *testing.T) {
			if test.mock != nil {
				test.mock()
			}
			err := s.api.refreshCSRFToken()
			if test.err != nil {
				assert.Error(t, err)
				assert.Nil(t, s.api.csrf)
			} else {
				assert.Equal(t, test.csrfToken, s.api.csrf.Value)
				assert.NoError(t, err)
			}
		})
	}
}
