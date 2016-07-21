package middleware_test

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"golang.org/x/net/context"

	"github.com/goadesign/goa"
	"github.com/goadesign/goa/middleware"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

// errorResponse contains the details of a error response. It implements ServiceError.
type errorResponse struct {
	// ID is the unique error instance identifier.
	ID string `json:"id" xml:"id" form:"id"`
	// Code identifies the class of errors.
	Code string `json:"code" xml:"code" form:"code"`
	// Status is the HTTP status code used by responses that cary the error.
	Status int `json:"status" xml:"status" form:"status"`
	// Detail describes the specific error occurrence.
	Detail string `json:"detail" xml:"detail" form:"detail"`
	// Meta contains additional key/value pairs useful to clients.
	Meta []map[string]interface{} `json:"meta,omitempty" xml:"meta,omitempty" form:"meta,omitempty"`
}

// Error returns the error occurrence details.
func (e *errorResponse) Error() string {
	msg := fmt.Sprintf("[%s] %d %s: %s", e.ID, e.Status, e.Code, e.Detail)
	for _, val := range e.Meta {
		for k, v := range val {
			msg += ", " + fmt.Sprintf("%s: %v", k, v)
		}
	}
	return msg
}

var _ = Describe("ErrorHandler", func() {
	var service *goa.Service
	var h goa.Handler
	var verbose bool

	var rw *testResponseWriter

	BeforeEach(func() {
		service = nil
		h = nil
		verbose = true
		rw = nil
	})

	JustBeforeEach(func() {
		rw = newTestResponseWriter()
		eh := middleware.ErrorHandler(service, verbose)(h)
		req, err := http.NewRequest("GET", "/foo", nil)
		Ω(err).ShouldNot(HaveOccurred())
		ctx := newContext(service, rw, req, nil)
		err = eh(ctx, rw, req)
		Ω(err).ShouldNot(HaveOccurred())
	})

	Context("with a handler returning a Go error", func() {

		BeforeEach(func() {
			service = newService(nil)
			h = func(ctx context.Context, rw http.ResponseWriter, req *http.Request) error {
				return errors.New("boom")
			}
		})

		It("turns Go errors into HTTP 500 responses", func() {
			Ω(rw.Status).Should(Equal(500))
			Ω(rw.ParentHeader["Content-Type"]).Should(Equal([]string{"text/plain"}))
			Ω(string(rw.Body)).Should(Equal(`"boom"` + "\n"))
		})

		Context("not verbose", func() {
			BeforeEach(func() {
				verbose = false
			})

			It("hides the error details", func() {
				var decoded errorResponse
				Ω(rw.Status).Should(Equal(500))
				Ω(rw.ParentHeader["Content-Type"]).Should(Equal([]string{goa.ErrorMediaIdentifier}))
				err := service.Decoder.Decode(&decoded, bytes.NewBuffer(rw.Body), "application/json")
				Ω(err).ShouldNot(HaveOccurred())
				msg := goa.ErrInternal(`Internal Server Error [zzz]`).Error()
				msg = regexp.QuoteMeta(msg)
				msg = strings.Replace(msg, "zzz", ".+", 1)
				endIDidx := strings.Index(msg, "]")
				msg = `\[.*\]` + msg[endIDidx+1:]
				Ω(fmt.Sprintf("%v", decoded.Error())).Should(MatchRegexp(msg))
			})
		})
	})

	Context("with a handler returning a goa error", func() {
		var gerr error

		BeforeEach(func() {
			service = newService(nil)
			gerr = goa.NewErrorClass("code", 418)("teapot", "foobar", 42)
			h = func(ctx context.Context, rw http.ResponseWriter, req *http.Request) error {
				return gerr
			}
		})

		It("maps goa errors to HTTP responses", func() {
			var decoded errorResponse
			Ω(rw.Status).Should(Equal(gerr.(goa.ServiceError).ResponseStatus()))
			Ω(rw.ParentHeader["Content-Type"]).Should(Equal([]string{goa.ErrorMediaIdentifier}))
			err := service.Decoder.Decode(&decoded, bytes.NewBuffer(rw.Body), "application/json")
			Ω(err).ShouldNot(HaveOccurred())
			Ω(decoded.Error()).Should(Equal(gerr.Error()))
		})
	})
})
