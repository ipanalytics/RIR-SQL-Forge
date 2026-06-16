package main

import (
	"context"
	"os"

	"github.com/ipanalytics/rir-sql-forge/internal/app"
)

func main() {
	os.Exit(app.Main(context.Background(), os.Args[1:], os.Stdout, os.Stderr))
}
