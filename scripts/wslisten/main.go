// Command wslisten connects to the realtime channel and prints every event it
// receives. It exists so the WebSocket can be verified from a terminal — before
// a demo, "is the socket actually pushing?" should take five seconds to answer,
// not require the iOS app.
//
//	go run ./scripts/wslisten -token "$ACCESS_TOKEN"
//	go run ./scripts/wslisten -token "$T" -url ws://localhost:8080/ws -for 30s
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/coder/websocket"
)

func main() {
	var (
		base     = flag.String("url", "ws://localhost:8080/ws", "websocket endpoint")
		token    = flag.String("token", "", "access token (required)")
		duration = flag.Duration("for", 0, "exit after this long (0 = until interrupted)")
		quiet    = flag.Bool("quiet", false, "print one line per event instead of full JSON")
	)
	flag.Parse()

	if *token == "" {
		fmt.Fprintln(os.Stderr, "wslisten: -token is required")
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if *duration > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *duration)
		defer cancel()
	}

	conn, _, err := websocket.Dial(ctx, *base+"?token="+*token, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "wslisten: dial failed: %v\n", err)
		os.Exit(1)
	}
	defer conn.CloseNow()
	conn.SetReadLimit(1 << 20)
	fmt.Fprintln(os.Stderr, "wslisten: connected")

	for {
		_, raw, err := conn.Read(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return // clean exit on timeout or Ctrl-C
			}
			fmt.Fprintf(os.Stderr, "wslisten: read failed: %v\n", err)
			os.Exit(1)
		}

		var envelope struct {
			Event string          `json:"event"`
			Data  json.RawMessage `json:"data"`
		}
		_ = json.Unmarshal(raw, &envelope)

		stamp := time.Now().Format("15:04:05")
		if *quiet {
			fmt.Printf("%s  %s\n", stamp, envelope.Event)
			continue
		}
		var pretty bytes.Buffer
		if err := json.Indent(&pretty, raw, "", "  "); err != nil {
			fmt.Printf("%s  %s\n", stamp, raw)
			continue
		}
		fmt.Printf("%s  %s\n%s\n", stamp, envelope.Event, pretty.String())
	}
}
