# Event Functions Go HTTP

This is a simple library in the style of
github.com/aws/aws-lambda-go/lambda to support handling CloudEvents
delivered over HTTP to a golang function.

## Usage:

```go
package main

import (
	"log"
	"context"
	"time"
	"github.com/evankanderson/efgh"
)

type MyData struct {
	AField string `json:"a_field,omitempty"`
	ANumber int64 `json:"a_number,omitempty"`
}

func HandleEvent(ctx context.Context, data MyData) error {
	eventTime, err := efgh.EventTime(ctx)
	if err != nil {
		eventTime = time.Now()
		log.Printf("Unable to read time from context\n")
	}
	eventId := efgh.EventId(ctx)
	log.Printf("Read event %s at %s: %r\n", eventId, eventTime.Format(time.RFC3339), data);
	return nil
}

func main() {
	efgh.Start(HandleEvent)
}
```

Internally, the framework supports any combination of context and data
as input, and output data and error in the response. If error is set,
an HTTP 500 will be returned.
