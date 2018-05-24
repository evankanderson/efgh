package efgh

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"reflect"
	"strings"
	"time"
)

// Implementation of HTTP processing for an event function.

type functionHandler struct {
	// The function in question
	f reflect.Value

	// metadata about the signature
	needsContext, hasError bool
	inType, outType        reflect.Type
}

func (fh functionHandler) Invoke(ctx context.Context, in []byte) (out []byte, err error) {
	var args []reflect.Value
	if fh.needsContext {
		args = append(args, reflect.ValueOf(ctx))
	}
	if fh.inType != nil {
		if fh.inType.Kind() == reflect.Slice && fh.inType.Elem().Kind() == reflect.Uint8 {
			args = append(args, reflect.ValueOf(in))
		} else if fh.inType.Kind() == reflect.Struct {
			data := reflect.New(fh.inType)
			if err = json.Unmarshal(in, data.Interface()); err != nil {
				log.Printf("Unable to unmarshall as JSON: %v\n", err)
				return nil, err
			}
			args = append(args, data.Elem())
		} else {
			return nil, fmt.Errorf("Unable to unmarshall type: %v", fh.inType)
		}
	}

	response := fh.f.Call(args)
	if fh.outType != nil {
		if fh.outType.Kind() == reflect.Slice && fh.outType.Elem().Kind() == reflect.Uint8 {
			out = response[0].Bytes()
		} else if fh.outType.Kind() == reflect.Struct {
			out, err = json.Marshal(response[0].Interface())
		} else {
			return nil, fmt.Errorf("Unable to marshal type: %v", fh.outType)
		}
	}
	if fh.hasError {
		var ok bool
		if err, ok = response[len(response)-1].Interface().(error); ok {
		}
	}

	return
}

func (fh functionHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		rw.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var in []byte
	var ctx context.Context
	var err error

	if strings.HasPrefix(req.Header.Get("Content-Type"), "application/cloudevents") {
		in, ctx, err = convertStructured(req)
	} else {
		in, ctx, err = convertBinary(req)
	}
	if err != nil {
		rw.Header().Set("Content-Type", "text/plain")
		rw.WriteHeader(http.StatusExpectationFailed)
		io.WriteString(rw, err.Error())
		return
	}
	// Handle different function types.
	out, err := fh.Invoke(ctx, in)
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		io.WriteString(rw, err.Error())
		return
	}
	// TODO: extract headers and/or handle binary vs structured on response.
	rw.WriteHeader(http.StatusOK)
	rw.Write(out)
}

// Handle Structed Content Mode (https://github.com/cloudevents/spec/blob/v0.1/http-transport-binding.md#32-structured-content-mode)
func convertStructured(req *http.Request) ([]byte, context.Context, error) {
	return nil, nil, errors.New("structured output not supported yet")
}

// Handle Binary Content Mode (https://github.com/cloudevents/spec/blob/v0.1/http-transport-binding.md#31-binary-content-mode)
func convertBinary(req *http.Request) ([]byte, context.Context, error) {
	cex := CloudEventContext{
		EventType:          req.Header.Get("CE-EventType"),
		EventTypeVersion:   req.Header.Get("CE-EventTypeVersion"),
		CloudEventsVersion: req.Header.Get("CE-CloudEventsVersion"),
		Source:             req.Header.Get("CE-Source"),
		EventID:            req.Header.Get("CE-EventID"),
		SchemaURL:          req.Header.Get("CE-SchemaURL"),
		ContentType:        req.Header.Get("Content-Type"),
	}
	ts := req.Header.Get("CE-EventTime")
	var err error
	if ts != "" {
		cex.EventTime, err = time.Parse(time.RFC3339, req.Header.Get("CE-EventTime"))
		if err != nil {
			return nil, nil, err
		}
	}
	// TODO: handle extensions

	ctx := context.WithValue(req.Context(), contextKey, cex)
	in, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return nil, nil, err
	}
	return in, ctx, nil
}

// Convert a function to an HTTP Handler
func wrap(function interface{}) (http.Handler, error) {
	h := functionHandler{
		f: reflect.ValueOf(function),
	}

	// Used to extract types
	ctxType := reflect.TypeOf((*context.Context)(nil)).Elem()
	errType := reflect.TypeOf((*error)(nil)).Elem()
	t := reflect.TypeOf(function)
	if t.Kind() != reflect.Func {
		return nil, fmt.Errorf("%v is not a function", t)
	}

	if t.NumIn() > 2 {
		return nil, fmt.Errorf("%v takes too many arguments", t)
	}
	if t.NumIn() > 0 {
		if t.In(0).Implements(ctxType) {
			h.needsContext = true
		} else {
			h.inType = t.In(0)
		}
	}
	if t.NumIn() == 2 {
		if !h.needsContext {
			return nil, fmt.Errorf("First argument must be of type context.Context: %v", t)
		}
		h.inType = t.In(1)
	}

	if t.NumOut() > 2 {
		return nil, fmt.Errorf("%v returns too many outputs", t)
	}
	if t.NumOut() > 0 {
		if t.Out(0).Implements(errType) {
			h.hasError = true
		} else {
			h.outType = t.Out(0)
		}
	}
	if t.NumOut() == 2 {
		if h.hasError || !t.Out(1).Implements(errType) {
			return nil, fmt.Errorf("Must return (data, error) with two arguments: %v", t)
		}
		h.hasError = true
	}

	return h, nil
}
