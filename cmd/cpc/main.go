package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/coffeedeveloper/cpc/internal/cpc"
)

var version = "dev"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := cpc.Run(ctx, os.Args[1:], version); err != nil {
		if cpc.IsCancelled(err) {
			os.Exit(130)
		}
		fmt.Fprintln(os.Stderr, "cpc:", err)
		os.Exit(1)
	}
}
