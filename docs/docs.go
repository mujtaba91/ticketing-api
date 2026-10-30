// Package docs embeds the swag-generated spec and a Swagger UI page, so the binary serves its own docs.
package docs

import "embed"

//go:embed swagger.json index.html
var FS embed.FS
