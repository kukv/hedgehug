package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/ops"
	"github.com/kukv/hedgehug/internal/store"
)

// Problem is the problem+json body of every error. Type is the Code of the
// ops error (task.not_found...), for the GUI to pick its words; Detail is the
// English message the CLI prints; Args are the arguments of the error.
type Problem struct {
	Type   string              `json:"type" doc:"What went wrong, such as task.not_found"`
	Title  string              `json:"title" doc:"The text of the HTTP status"`
	Status int                 `json:"status"`
	Detail string              `json:"detail,omitempty" doc:"The message in English, as the CLI prints it"`
	Args   map[string]any      `json:"args,omitempty" doc:"The arguments of the error, by name"`
	Errors []*huma.ErrorDetail `json:"errors,omitempty" doc:"What was wrong in the request, for request.invalid"`
}

func (p *Problem) Error() string             { return p.Detail }
func (p *Problem) GetStatus() int            { return p.Status }
func (p *Problem) ContentType(string) string { return "application/problem+json" }

func init() { huma.NewError = newHumaError }

// newHumaError is the error huma makes itself: a request it cannot read or
// that fails its checks is request.invalid (400), whatever status huma
// chose; the others are typed by their status.
func newHumaError(status int, msg string, errs ...error) huma.StatusError {
	p := &Problem{Status: status, Detail: msg}
	for _, e := range errs {
		if e == nil {
			continue
		}
		var d huma.ErrorDetailer
		if errors.As(e, &d) {
			p.Errors = append(p.Errors, d.ErrorDetail())
		} else {
			p.Errors = append(p.Errors, &huma.ErrorDetail{Message: e.Error()})
		}
	}
	switch {
	case status == http.StatusBadRequest || status == http.StatusUnprocessableEntity:
		p.Status, p.Type = http.StatusBadRequest, "request.invalid"
	case status >= 500:
		p.Type = "internal"
	default:
		p.Type = "http." + strconv.Itoa(status)
	}
	p.Title = http.StatusText(p.Status)
	return p
}

// problemOf is the problem of an error an operation returned.
func problemOf(err error) *Problem {
	e := ops.AsError(err)
	p := &Problem{Type: e.Code, Detail: detailOf(err), Args: e.Args}
	switch e.Kind {
	case ops.KindNotFound:
		p.Status = http.StatusNotFound
	case ops.KindAmbiguous, ops.KindConflict:
		p.Status = http.StatusConflict
	case ops.KindInvalid:
		p.Status = http.StatusUnprocessableEntity
	case ops.KindUsage:
		p.Status = http.StatusBadRequest
	default:
		p.Status, p.Type, p.Args = http.StatusInternalServerError, "internal", nil
	}
	if p.Type == "" {
		p.Type = "internal"
	}
	var ve *store.ValidationError
	if errors.As(err, &ve) {
		vs := make([]map[string]string, len(ve.Violations))
		for i, v := range ve.Violations {
			vs[i] = map[string]string{"code": v.Code, "field": v.Field, "message": v.Msg}
		}
		if p.Args == nil {
			p.Args = map[string]any{}
		}
		p.Args["violations"] = vs
	}
	p.Title = http.StatusText(p.Status)
	return p
}

// detailOf is the message of err as the CLI prints it after "hedgehug: "
// (cmd/hedgehug describe).
func detailOf(err error) string {
	var ve *store.ValidationError
	if errors.As(err, &ve) {
		return ops.ViolationsText(ve)
	}
	if errors.Is(err, store.ErrLocked) {
		return "another hedgehug is writing to the store; try again"
	}
	return err.Error()
}

// writeProblem writes p outside of huma (the Host and token checks, static
// files).
func writeProblem(w http.ResponseWriter, p *Problem) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(p.Status)
	_ = json.NewEncoder(w).Encode(p)
}

// newProblem is a problem of status with type typ and detail msg.
func newProblem(status int, typ, msg string) *Problem {
	return &Problem{Type: typ, Title: http.StatusText(status), Status: status, Detail: msg}
}
