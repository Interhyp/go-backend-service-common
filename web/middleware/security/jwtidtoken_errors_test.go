package security

import (
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	"github.com/Interhyp/go-backend-service-common/docs"
	"github.com/golang-jwt/jwt/v4"
	"github.com/stretchr/testify/require"
)

func TestJwtIdTokenValidatorMiddleware_ExpiredWithLaterNonMatchingKey_ReportsExpiry(t *testing.T) {
	docs.Description("jwt middleware reports expiry even if a later key fails the signature check")
	signing, other := tstGenerateKey(t), tstGenerateKey(t)

	_, err := validateToken(tstSignToken(t, signing, -time.Minute), []*rsa.PublicKey{&signing.PublicKey, &other.PublicKey})

	require.Error(t, err)
	require.Regexp(t, `^token is expired by `, describeTokenError(err))
}

func TestJwtIdTokenValidatorMiddleware_ExpiredWithEarlierNonMatchingKey_ReportsExpiry(t *testing.T) {
	docs.Description("jwt middleware reports expiry even if an earlier key fails the signature check")
	signing, other := tstGenerateKey(t), tstGenerateKey(t)

	_, err := validateToken(tstSignToken(t, signing, -time.Minute), []*rsa.PublicKey{&other.PublicKey, &signing.PublicKey})

	require.Error(t, err)
	require.Regexp(t, `^token is expired by `, describeTokenError(err))
}

func TestJwtIdTokenValidatorMiddleware_ValidWithMultipleKeys_Ok(t *testing.T) {
	docs.Description("jwt middleware accepts a token signed by any of the configured keys")
	signing, other := tstGenerateKey(t), tstGenerateKey(t)

	claims, err := validateToken(tstSignToken(t, signing, time.Hour), []*rsa.PublicKey{&other.PublicKey, &signing.PublicKey})

	require.NoError(t, err)
	require.Equal(t, "John Doe", claims.Name)
}

func TestJwtIdTokenValidatorMiddleware_InvalidSignature_DescribesError(t *testing.T) {
	docs.Description("jwt middleware describes a signature mismatch in words instead of an error bitmask")
	signing, other := tstGenerateKey(t), tstGenerateKey(t)

	_, err := validateToken(tstSignToken(t, signing, time.Hour), []*rsa.PublicKey{&other.PublicKey})

	require.Equal(t, "token signature is invalid: crypto/rsa: verification error", describeTokenError(err))
}

func TestJwtIdTokenValidatorMiddleware_Malformed_DescribesError(t *testing.T) {
	docs.Description("jwt middleware describes a malformed token")
	key := tstGenerateKey(t)

	_, err := validateToken("not-a-token", []*rsa.PublicKey{&key.PublicKey})

	require.Equal(t, "token is malformed: token contains an invalid number of segments", describeTokenError(err))
}

func TestJwtIdTokenValidatorMiddleware_NoKeys_DescribesError(t *testing.T) {
	docs.Description("jwt middleware reports missing key configuration")
	_, err := validateToken("any", nil)

	require.Equal(t, "token is unverifiable: no public keys configured", describeTokenError(err))
}

// --- helpers ---

func tstGenerateKey(t *testing.T) *rsa.PrivateKey {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	return key
}

func tstSignToken(t *testing.T, key *rsa.PrivateKey, expiresIn time.Duration) string {
	claims := AllClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "myissuer",
			Subject:   "1234567890",
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiresIn)),
		},
		CustomClaims: CustomClaims{
			Name:   "John Doe",
			Email:  "john.doe@example.com",
			Groups: []string{"somegroup"},
		},
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(key)
	require.NoError(t, err)
	return signed
}
