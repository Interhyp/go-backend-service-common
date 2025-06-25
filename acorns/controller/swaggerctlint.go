package controller

import (
	"context"
	"github.com/go-chi/chi/v5"
)

const SwaggerControllerAcornName = "swaggerctl"

// SwaggerController provides the swagger ui and serves our openapi v3 spec from docs/openapi-v3-spec.json
type SwaggerController interface {
	IsSwaggerController() bool

	WireUp(ctx context.Context, router chi.Router)
}

// SpecFile describes an OpenApi spec file served by this controller.
// fields:
//   - RelativeFilesystemPath where to find the file(s) to serve relative to the current working directory
//     example: "docs"
//     note: do NOT add a trailing slash
//   - FileName which exact file to serve. This will be added to the route, so only exactly this file is made available
//     example: "swagger.json"
//   - UriPath under which path the file should be served. Must start with "/"
//     example: "/"
//     Note: unfortunately it is not possible to use a different filename, as this is a direct filesystem directory server.
//     That's why we have the redirect.
type SpecFile struct {
	RelativeFilesystemPath string
	FileName               string
	UriPath                string
}
