// Package docs embeds the hand-written OpenAPI spec and a Swagger UI page, so the binary serves its own docs.
package docs

import "embed"

//go:embed openapi.yaml index.html
var FS embed.FS
