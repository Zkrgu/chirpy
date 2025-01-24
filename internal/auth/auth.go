package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	return string(bytes), err
}

func CheckPasswordHash(password, hash string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

func MakeJWT(userId uuid.UUID, tokenSecret string, expiresIn time.Duration) (string, error) {
	return jwt.NewWithClaims(
		jwt.SigningMethodHS256,
		jwt.RegisteredClaims{
			Issuer:    "chirpy",
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiresIn)),
			Subject:   userId.String(),
		},
	).SignedString([]byte(tokenSecret))
}

func ValidateJWT(tokenString, tokenSecret string) (uuid.UUID, error) {
	claims := jwt.RegisteredClaims{}
	token, err := jwt.ParseWithClaims(tokenString, &claims, func(token *jwt.Token) (interface{}, error) { return []byte(tokenSecret), nil })
	if err != nil {
		return uuid.Nil, err
	}
	sub, err := token.Claims.GetSubject()
	if err != nil {
		return uuid.Nil, err
	}
	id, err := uuid.Parse(sub)
	return id, err
}

func GetBearerToken(headers http.Header) (string, error) {
	authHeader := headers.Get("Authorization")
	if len(authHeader) == 0 {
		return authHeader, fmt.Errorf("Empty Authorization header")
	}
	token, found := strings.CutPrefix(authHeader, "Bearer ")
	if !found {
		return "", fmt.Errorf("Not a Bearer token")
	}
	return token, nil
}

func MakeRefreshToken() (string, error) {
	bytes := make([]byte, 32)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func GetApiKey(headers http.Header) (string, error) {
	authHeader := headers.Get("Authorization")
	if len(authHeader) == 0 {
		return authHeader, fmt.Errorf("Empty Authorization header")
	}
	token, found := strings.CutPrefix(authHeader, "ApiKey ")
	if !found {
		return "", fmt.Errorf("Not a ApiKey")
	}
	return token, nil
}
