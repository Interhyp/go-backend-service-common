package security

import (
	"crypto/rsa"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-http-utils/headers"
	"github.com/golang-jwt/jwt/v4"
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

				claims, err := validateToken(tokenString, options.PublicKeys)
				if err != nil {
					unauthorizedErrorHandler(ctx, w, r, describeTokenError(err), Now())
					return
				}

				ctx = PutRawToken(ctx, tokenString)
				ctx = PutClaims(ctx, claims)
				next.ServeHTTP(w, r.WithContext(ctx))
			}
		}
		return http.HandlerFunc(fn)
	}
	return mw
}

func validateToken(tokenString string, keys []*rsa.PublicKey) (*AllClaims, error) {
	if len(keys) == 0 {
		return nil, fmt.Errorf("%w: no public keys configured", jwt.ErrTokenUnverifiable)
	}

	var mostRelevantErr error
	for _, key := range keys {
		claims, err := validateTokenWithKey(tokenString, key)
		if err == nil {
			return claims, nil
		}
		// an error from a key whose signature matched (e.g. expired) is more relevant than a signature mismatch
		if mostRelevantErr == nil || (isSignatureError(mostRelevantErr) && !isSignatureError(err)) {
			mostRelevantErr = err
		}
	}
	return nil, mostRelevantErr
}

func validateTokenWithKey(tokenString string, key *rsa.PublicKey) (*AllClaims, error) {
	claims := &AllClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, keyFuncForKey(key), jwt.WithValidMethods([]string{"RS256"}))
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, errors.New("token parsed but invalid")
	}
	return claims, nil
}

func isSignatureError(err error) bool {
	return errors.Is(err, jwt.ErrTokenSignatureInvalid)
}

var describedTokenErrors = []error{
	jwt.ErrTokenMalformed,
	jwt.ErrTokenUnverifiable,
	jwt.ErrTokenSignatureInvalid,
	jwt.ErrTokenExpired,
	jwt.ErrTokenNotValidYet,
	jwt.ErrTokenUsedBeforeIssued,
	jwt.ErrTokenInvalidClaims,
}

// describeTokenError makes sure the message names the kind of failure, e.g.
// "token signature is invalid: crypto/rsa: verification error" instead of just "crypto/rsa: verification error"
func describeTokenError(err error) string {
	message := err.Error()
	for _, kind := range describedTokenErrors {
		if errors.Is(err, kind) {
			if strings.HasPrefix(message, kind.Error()) {
				return message
			}
			return fmt.Sprintf("%s: %s", kind.Error(), message)
		}
	}
	return message
}

func keyFuncForKey(rsaPublicKey *rsa.PublicKey) func(token *jwt.Token) (interface{}, error) {
	return func(token *jwt.Token) (interface{}, error) {
		return rsaPublicKey, nil
	}
}
