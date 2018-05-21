package efgh

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"reflect"
	"time"
)

// key is an unexported type for keys defined in this package.
// This prevents collisions with keys defined in other packages.
type key int

// headerKey is the key for http.Header values in Contexts. clients
// use specific accessors like `EventTime` instead of using this key
// directly.
var headerKey key

func Start(function interface{}) {
	handler, err := wrap(function)
	if err != nil {
		log.Fatal(err)
	}
	http.Handle("/", handler)
	port := ":" + os.Getenv("PORT")
	log.Printf("Listening on %s\n", port)
	log.Fatal(http.ListenAndServe(port, nil))
}

func wrap(function interface{}) (http.Handler, error) {
	h := FunctionHandler{
		f: reflect.ValueOf(function),
	}

	// Used to extract types
	ctxType := reflect.TypeOf((*context.Context)(nil)).Elem()
	errType := reflect.TypeOf((*error)(nil)).Elem()
	t := reflect.TypeOf(function)
	if t.Kind() != reflect.Func {
		return nil, fmt.Errorf("%r is not a function", t)
	}

	if t.NumIn() > 2 {
		return nil, fmt.Errorf("%r takes too many arguments", t)
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
			return nil, fmt.Errorf("First argument must be of type context.Context: %r", t)
		}
		h.inType = t.In(1)
	}

	if t.NumOut() > 2 {
		return nil, fmt.Errorf("%r returns too many outputs", t)
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
			return nil, fmt.Errorf("Must return (data, error) with two arguments: %r", t)
		}
		h.hasError = true
	}

	return h, nil
}

func (fh FunctionHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		rw.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	ctx := req.Context()
	ctx = context.WithValue(ctx, headerKey, req.Header)
	in, err := ioutil.ReadAll(req.Body)
	//	var out []byte
	if err != nil {
		rw.WriteHeader(http.StatusExpectationFailed)
		return
	}
	// Handle different function types.
	out, err := fh.Invoke(ctx, in)
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		io.WriteString(rw, err.Error())
		return
	}
	rw.WriteHeader(http.StatusOK)
	rw.Write(out)
}

func (fh FunctionHandler) Invoke(ctx context.Context, in []byte) (out []byte, err error) {
	var args []reflect.Value
	if fh.needsContext {
		args = append(args, reflect.ValueOf(ctx))
	}
	if fh.inType != nil {
		data := reflect.New(fh.inType)
		if fh.inType.Kind() == reflect.Struct {
			log.Printf("Decoding '%s' to struct\n", in)
			if err = json.Unmarshal(in, data.Interface()); err != nil {
				log.Printf("Unable to unmarshall: %r\n", err)
				return
			}
		} else if fh.inType.Kind() == reflect.Array && fh.inType.Elem().Kind() == reflect.Uint8 {
			data.SetBytes(in)
		}

		args = append(args, data.Elem())
		log.Printf("Data is: %s (%s/%s)\n", data.String(), fh.inType.Kind(), data.Interface())
	}
	log.Printf("Calling with args: %r\n", args)
	response := fh.f.Call(args)
	i := 0
	if fh.outType != nil {
		// TODO: unpack type
		log.Printf("Should unpack %r", response[i])
		i++
	}
	if fh.hasError {
		var ok bool
		if err, ok = response[i].Interface().(error); ok {
		} else {
			log.Printf("Type of err(%d) is %s\n", i, response[i].Type())
			//			err = fmt.Errorf("Could not extract error from %r", response)
		}
	}

	return
}

func EventTime(ctx context.Context) (time.Time, error) {
	ts := getHeader(ctx, "Ce-Eventtime")
	return time.Parse(time.RFC3339, ts)
}

func EventId(ctx context.Context) string {
	return getHeader(ctx, "Ce-Eventid")
}

func getHeader(ctx context.Context, key string) string {
	h, ok := ctx.Value(headerKey).(http.Header)
	if !ok {
		return ""
	}
	return h.Get(key)
}

type FunctionHandler struct {
	// The function in question
	f reflect.Value

	// metadata about the signature
	needsContext, hasError bool
	inType, outType        reflect.Type
}
