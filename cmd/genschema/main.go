// cmd/genschema generates the static OpenAPI schema.json from the runtime registry.
//
// Usage: go run ./cmd/genschema/ > openapi/schema.json
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/forklifts-for-great-justice/judge-service/internal/handlers"
	"github.com/forklifts-for-great-justice/judge-service/internal/openapi"
)

func main() {
	reg := openapi.NewRegistry()

	handlers.RegisterHealthOpenAPI(reg)
	handlers.RegisterOpenAPI(reg)

	spec := reg.Spec()

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(spec); err != nil {
		fmt.Fprintf(os.Stderr, "encoding error: %v\n", err)
		os.Exit(1)
	}
}
