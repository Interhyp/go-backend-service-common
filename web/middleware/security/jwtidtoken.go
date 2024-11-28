package security

import (
	"crypto/rsa"
	"errors"
	"fmt"
	"github.com/go-http-utils/headers"
	"github.com/golang-jwt/jwt/v4"
	"net/http"
	"strings"
)

type JwtIdTokenValidatorMiddlewareOptions struct {
	PublicKeys []*rsa.PublicKey
}

// ParsePublicKeysFromPEM is a helper function to parse RSA public keys in PEM format
func ParsePublicKeysFromPEM(publicKeyPEMs []string) ([]*rsa.PublicKey, error) {
	var rsaPublicKeys = make([]*rsa.PublicKey, 0)

	for _, publicKeyPEM := range publicKeyPEMs {
		publicKey, err := jwt.ParseRSAPublicKeyFromPEM([]byte(publicKeyPEM))
		if err != nil {
			return rsaPublicKeys, err
		}

		rsaPublicKeys = append(rsaPublicKeys, publicKey)
	}

	return rsaPublicKeys, nil
}

func JwtIdTokenValidatorMiddleware(options JwtIdTokenValidatorMiddlewareOptions) func(http.Handler) http.Handler {
	mw := func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			authHeaderValue := r.Header.Get(headers.Authorization)
			const bearerPrefix = "Bearer "
			if !strings.HasPrefix(authHeaderValue, bearerPrefix) {
				// valid case, no bearer authorization provided
				next.ServeHTTP(w, r)
			} else {
				ctx := r.Context()
				tokenString := strings.TrimSpace(strings.TrimPrefix(authHeaderValue, bearerPrefix))

				errorMessage := ""
				for _, key := range options.PublicKeys {
					claims := AllClaims{}
					token, err := jwt.ParseWithClaims(tokenString, &claims, keyFuncForKey(key), jwt.WithValidMethods([]string{"RS256"}))
					if err == nil && token.Valid {
						parsedClaims := token.Claims.(*AllClaims)

						ctx = PutRawToken(ctx, token.Raw)
						ctx = PutClaims(ctx, parsedClaims)
						next.ServeHTTP(w, r.WithContext(ctx))
						return
					}
					if err != nil {
						errorMessage = err.Error()
						var vErr *jwt.ValidationError
						if errors.As(err, &vErr) {
							errorMessage = fmt.Sprintf("%s: %d", errorMessage, vErr.Errors)
						}
					} else if !token.Valid {
						errorMessage = "token parsed but invalid"
					}
				}
				unauthorizedErrorHandler(ctx, w, r, errorMessage, Now())
			}
		}
		return http.HandlerFunc(fn)
	}
	return mw
}

func keyFuncForKey(rsaPublicKey *rsa.PublicKey) func(token *jwt.Token) (interface{}, error) {
	return func(token *jwt.Token) (interface{}, error) {
		return rsaPublicKey, nil
	}
}
