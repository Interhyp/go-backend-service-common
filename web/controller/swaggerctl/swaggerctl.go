package swaggerctl

import (
	"context"
	"fmt"
	"github.com/Interhyp/go-backend-service-common/acorns/controller"
	"github.com/Interhyp/go-backend-service-common/web/util/media"
	aulogging "github.com/StephanHCB/go-autumn-logging"
	auwebswaggerui "github.com/StephanHCB/go-autumn-web-swagger-ui"
	"github.com/go-chi/chi/v5"
	"github.com/go-http-utils/headers"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type SwaggerCtlImpl struct {
	additionalSpecFiles []controller.SpecFile
}

func (c *SwaggerCtlImpl) WireUp(ctx context.Context, router chi.Router) {
	// 	serve swagger-ui and openapi spec json (which needs to be in the file system of your container)
	c.AddStaticHttpFilesystemRoute(router, auwebswaggerui.Assets, "/swagger-ui")
	openApiSpecFile, fileFindError := c.GetFirstMatchingServableFile([]string{"docs", "api"}, regexp.MustCompile(`openapi-v3-spec\.(json|yaml)`))
	if fileFindError != nil {
		aulogging.Logger.NoCtx().Error().Print("failed to find openAPI spec file. OpenAPI spec will be unavailable.")
	}
	if err := c.AddStaticFileRoute(router, openApiSpecFile); fileFindError == nil && err != nil {
		aulogging.Logger.NoCtx().Error().Printf("failed to read openAPI spec file %s/%s. OpenAPI spec will be unavailable.", openApiSpecFile.RelativeFilesystemPath, openApiSpecFile.FileName)
	}

	for _, additionalFile := range c.additionalSpecFiles {
		if err := c.AddStaticFileRoute(router, additionalFile); fileFindError == nil && err != nil {
			aulogging.Logger.NoCtx().Error().Printf("failed to read spec file %s/%s. OpenAPI spec will be broken.", additionalFile.RelativeFilesystemPath, additionalFile.FileName)
		}
	}

	c.AddRedirect(router, "/v3/api-docs", fmt.Sprintf("/%s", openApiSpecFile.FileName))
}

// serve static files from an http.FileSystem instance via a chi router
//
// inspired by https://github.com/go-chi/chi/blob/master/_examples/fileserver/main.go
//
// parameters:
//   - uriPath under which sub-url they should be served, if you leave out trailing slash, also adds a redirect
//     example: "/swagger-ui"
func (c *SwaggerCtlImpl) AddStaticHttpFilesystemRoute(server chi.Router, fs http.FileSystem, uriPath string) {
	strippedFs := http.StripPrefix(uriPath, http.FileServer(fs))

	if hasNoTrailingSlash(uriPath) {
		server.Get(uriPath, http.RedirectHandler(uriPath+"/", 301).ServeHTTP)
		uriPath += "/"
	}
	uriPath += "*"

	server.Get(uriPath, func(w http.ResponseWriter, r *http.Request) {
		strippedFs.ServeHTTP(w, r)
	})
}

// serve a single static file via a chi router
//
// parameters:
//   - specFile describes the spec file to serve. UriPath defaults to "/" if empty.
func (c *SwaggerCtlImpl) AddStaticFileRoute(server chi.Router, specFile controller.SpecFile) error {
	workDir, _ := os.Getwd()
	filePath := filepath.Join(workDir, specFile.RelativeFilesystemPath, specFile.FileName)

	contents, err := os.ReadFile(filePath)
	if err != nil {
		aulogging.Logger.NoCtx().Info().WithErr(err).Printf("failed to read file %s - skipping: %s", filePath, err.Error())
		return err
	}

	if hasNoTrailingSlash(specFile.UriPath) {
		specFile.UriPath = specFile.UriPath + "/"
	}

	server.Get(specFile.UriPath+specFile.FileName, func(w http.ResponseWriter, r *http.Request) {
		// this stops browsers from caching our swagger.json
		w.Header().Set(headers.CacheControl, "no-cache")
		w.Header().Set(headers.ContentType, media.ContentTypeApplicationJson)
		_, _ = w.Write(contents)
	})

	return nil
}

// returns the first file from workdir whose path match a given regex
//
// parameters:
//   - fileMatcher a regular expression used to filter the files
func (c *SwaggerCtlImpl) GetFirstMatchingServableFile(relativeFilesystemPaths []string, fileMatcher *regexp.Regexp) (controller.SpecFile, error) {
	workDir, _ := os.Getwd()
	for _, relativeFilesystemPath := range relativeFilesystemPaths {
		dirPath := filepath.Join(workDir, relativeFilesystemPath)

		contents, err := os.ReadDir(dirPath)
		if err != nil {
			aulogging.Logger.NoCtx().Info().WithErr(err).Printf("failed to read directory %s - skipping directory", dirPath)
			continue
		}

		for _, element := range contents {
			if !element.IsDir() && fileMatcher != nil && fileMatcher.MatchString(element.Name()) {
				return controller.SpecFile{
					RelativeFilesystemPath: relativeFilesystemPath,
					FileName:               element.Name(),
				}, nil
			}
		}

	}
	return controller.SpecFile{}, fmt.Errorf("no file matching %s found in relative paths %s", fileMatcher.String(), strings.Join(relativeFilesystemPaths, ", "))
}

func hasNoTrailingSlash(path string) bool {
	return len(path) == 0 || path[len(path)-1] != '/'
}

func (c *SwaggerCtlImpl) AddRedirect(server chi.Router, source string, target string) {
	server.Get(source, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target, http.StatusMovedPermanently)
	})
}
