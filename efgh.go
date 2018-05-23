package efgh

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"
)

// Start instantiates a webserver to surface the supplied function
// as an HTTP endpoint implementing the
// [Cloud Events HTTP transport](https://github.com/cloudevents/spec/blob/v0.1/http-transport-binding.md#31-binary-content-mode).
//
// Start takes a flexible range of function signatures; any combination
// of the following should work for input args:
//
// ```
// func DoIt()
// func DoIt(context.Context)
// func DoIt([]bytes)
// func DoIt(interface{})  // For JSON unmarshal
// func DoIt(context.Context, [] bytes)
// func DoIt(context.Context, interface{})  // For JSON unmarshal
// ```
//
// Similarly, the return value of the function may be any of:
//
// ```
// func DoIt()
// func DoIt() error
// func DoIt() []byte
// func DoIt() interface{}  // For JSON marshall
// func DoIt() (error, []byte)
// func DoIt() (error, interface{})  // For JSON marshall
// ```
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

// CloudEventContext represents the CloudEvents "context" fields.
type CloudEventContext struct {
	// Type of occurence which has happened.
	EventType string `json:"eventType"`
	// The version of the `eventType`; this is producer-specific.
	EventTypeVersion string `json:"eventTypeVersion,omitempty"`
	// The version of the CloudEVents specification used by the event.
	CloudEventsVersion string `json:"cloudEventsVersion"`
	// The event producer; this is a URI, but exact syntax is producer-specific.
	Source string `json:"source"`
	// ID of the event; must be non-empty and unique within the scope of the producer.
	EventID string `json:"eventID"`
	// Timestamp of when the event happened.
	EventTime time.Time `json:"eventTime,omitempty"`
	// A link to the schema that the `data` attribute adheres to.
	SchemaURL string `json:"schemaURL,omitempty"`
	// Describes the data encoding format.
	ContentType string `json:"contentType,omitempty"`
	// Additional metadata without a well-defined structure.
	Extensions map[string]json.RawMessage `json:"extensions,omitempty"`
}

// CloudEvent extracts a CloudEventContext from the current context.
func CloudEvent(ctx context.Context) (CloudEventContext, bool) {
	r, ok := ctx.Value(contextKey).(CloudEventContext)
	return r, ok
}

// key is an unexported type for keys defined in this package.
// This prevents collisions with keys defined in other packages.
type key int

// contextKey is the key for CloudEventContext values in Contexts.
var contextKey key
