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
	AField string `json: "a_field"`
	ANumber int64 `json: "a_number"`
}

func HandleEvent(ctx context.Context, data MyData) error {
	eventTime := efgh.EventTime(ctx)
	eventId := efgh.EventId(ctx)
	log.Printf("Read event %s at %s: %r", eventId, eventTime.Format(time.RFC3339), data);
}

func main() {
	efgh.Start(HandleEvent)
}
```

Internally, the framework supports any combination of context and data
as input, and output data and error in the response. If error is set,
an HTTP 500 will be returned.
