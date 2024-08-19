package middleware

import (
	"context"
	"github.com/Interhyp/go-backend-service-common/web/middleware/apmtracing"
	"github.com/Interhyp/go-backend-service-common/web/middleware/cancellogger"
	"github.com/Interhyp/go-backend-service-common/web/middleware/corsheader"
	"github.com/Interhyp/go-backend-service-common/web/middleware/recoverer"
	"github.com/Interhyp/go-backend-service-common/web/middleware/requestid"
	"github.com/Interhyp/go-backend-service-common/web/middleware/requestidinresponse"
	"github.com/Interhyp/go-backend-service-common/web/middleware/requestlogging"
	"github.com/Interhyp/go-backend-service-common/web/middleware/requestmetrics"
	"github.com/Interhyp/go-backend-service-common/web/middleware/security"
	"github.com/Interhyp/go-backend-service-common/web/middleware/timeout"
	aulogging "github.com/StephanHCB/go-autumn-logging"
	"github.com/StephanHCB/go-autumn-logging-zerolog/loggermiddleware"
	auapmmiddleware "github.com/StephanHCB/go-autumn-restclient-apm/implementation/middleware"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

type MiddlewareStackOptions struct {
	ElasticApmEnabled bool
	PlainLogging      bool

	CorsAllowOrigin string // set to enable

	RequestTimeoutSeconds int // set >0 to enable

	HasJwtIdTokenAuthorization bool
	JwtPublicKeyPEMs           []string

	// support a fixed basic auth setup for use by e.g. CI systems
	//
	// can be used both in addition to HasJwtAuthorization and standalone
	//
	// to enable, set HasBasicAuthAuthorization and provide nonempty username and password from
	// configuration. When the authorization header matches, the injected user will then have
	// the CustomClaims provided here set in the request context.
	HasBasicAuthAuthorization bool
	BasicAuthUsername         string
	BasicAuthPassword         string
	BasicAuthClaims           security.CustomClaims

	DisableSecurityEnforcement bool
	// AllowUnauthorized is the explicit list of method + url path combinations that allow unauthorized access.
	//
	// We perform a regular expression match against the capitalized HTTP method, followed by 1 space,
	// followed by the absolute URL path. Start and end markers are added to the regexp under the hood, so
	// the example actually matches against "^PUT /v1/info$". Regexp quoting rules apply as usual.
	//
	// examples: "PUT /v1/info", "GET /swagger-ui.*" (regexp supported)
	AllowUnauthorized []string

	// SkipDuplicateSetup must be set to true if you are trying to set up a second middleware stack
	//
	// Some setup cannot be run multiple times, e.g. metrics registration panics on duplicates
	SkipDuplicateSetup bool

	RequestLoggingOptions requestlogging.Options
}

func SetupStandardMiddlewareStack(ctx context.Context, router chi.Router, options MiddlewareStackOptions) error {
	router.Use(cancellogger.ConstructContextCancellationLoggerMiddleware("Top"))

	router.Use(requestid.RequestID)
	router.Use(cancellogger.ConstructContextCancellationLoggerMiddleware("RequestID"))

	tracingOptions := apmtracing.ApmMiddlewareOptions{
		ElasticApmEnabled: options.ElasticApmEnabled,
		PlainLogging:      options.PlainLogging,
	}
	if options.SkipDuplicateSetup {
		tracingMiddleware := apmtracing.GetApmMiddlewareAfterSetup(ctx, tracingOptions)
		router.Use(tracingMiddleware)
	} else {
		tracingMiddleware := apmtracing.BuildApmMiddleware(ctx, tracingOptions)
		router.Use(tracingMiddleware)
	}
	router.Use(cancellogger.ConstructContextCancellationLoggerMiddleware("ElasticApm"))

	loggerOptions := apmtracing.ConfigureContextLoggingForApm(ctx, tracingOptions)
	loggermiddleware.MethodFieldName = "http.request.method"
	loggermiddleware.PathFieldName = "url.path"
	router.Use(loggermiddleware.AddZerologLoggerToContextMiddleware(loggerOptions))
	router.Use(cancellogger.ConstructContextCancellationLoggerMiddleware("AddZerologLogger"))

	if !options.SkipDuplicateSetup {
		requestlogging.Setup(options.RequestLoggingOptions)
	}
	router.Use(chimiddleware.Logger)
	router.Use(cancellogger.ConstructContextCancellationLoggerMiddleware("Logger"))

	router.Use(recoverer.PanicRecoverer)
	router.Use(cancellogger.ConstructContextCancellationLoggerMiddleware("PanicRecoverer"))

	router.Use(requestidinresponse.AddRequestIdHeaderToResponse)
	router.Use(cancellogger.ConstructContextCancellationLoggerMiddleware("AddRequestIdHeaderToResponse"))

	router.Use(auapmmiddleware.AddTraceHeadersToResponse)
	router.Use(cancellogger.ConstructContextCancellationLoggerMiddleware("AddTraceHeadersToResponse"))

	router.Use(corsheader.CorsHandlingWithCorsAllowOrigin(options.CorsAllowOrigin))
	router.Use(cancellogger.ConstructContextCancellationLoggerMiddleware("CorsHandling"))

	if !options.SkipDuplicateSetup {
		requestmetrics.Setup() // panics if called twice
	}
	router.Use(requestmetrics.RecordRequestMetrics)
	router.Use(cancellogger.ConstructContextCancellationLoggerMiddleware("RecordRequestMetrics"))

	if options.HasJwtIdTokenAuthorization {
		rsaKeys, err := security.ParsePublicKeysFromPEM(options.JwtPublicKeyPEMs)
		if err != nil {
			// breaking because the service probably will not work correctly without its key set anyway
			aulogging.Logger.Ctx(ctx).Error().WithErr(err).Printf("Failed to parse PEM public keys for JWT validation - bailing out: %s", err.Error())
			return err
		}
		jwtOptions := security.JwtIdTokenValidatorMiddlewareOptions{
			PublicKeys: rsaKeys,
		}
		router.Use(security.JwtIdTokenValidatorMiddleware(jwtOptions))
		router.Use(cancellogger.ConstructContextCancellationLoggerMiddleware("JwtIdTokenValidator"))
	}
	if options.HasBasicAuthAuthorization {
		basicAuthOptions := security.BasicAuthMiddlewareOptions{
			BasicAuthUsername: options.BasicAuthUsername,
			BasicAuthPassword: options.BasicAuthPassword,
			BasicAuthClaims:   options.BasicAuthClaims,
		}
		router.Use(security.BasicAuthValidatorMiddleware(basicAuthOptions))
		router.Use(cancellogger.ConstructContextCancellationLoggerMiddleware("BasicAuthValidator"))
	}
	if !options.DisableSecurityEnforcement {
		allowThroughOptions := security.AuthRequiredMiddlewareOptions{
			AllowUnauthorized: options.AllowUnauthorized,
		}
		router.Use(security.AuthRequiredMiddleware(allowThroughOptions))
		router.Use(cancellogger.ConstructContextCancellationLoggerMiddleware("AuthRequired"))
	}

	if options.RequestTimeoutSeconds > 0 {
		if !options.SkipDuplicateSetup {
			timeout.RequestTimeoutSeconds = options.RequestTimeoutSeconds
		} else if timeout.RequestTimeoutSeconds != options.RequestTimeoutSeconds {
			aulogging.Logger.Ctx(ctx).Warn().Print("Failed to set up request timeout for second web stack - timeouts cannot differ - timeout for both stacks remains set to %d seconds - continuing", timeout.RequestTimeoutSeconds)
		}
		router.Use(timeout.AddRequestTimeout)
		router.Use(cancellogger.ConstructContextCancellationLoggerMiddleware("AddRequestTimeout"))
	}

	return nil
}
