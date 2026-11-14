package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/deb151292/neurealm-go-Assignment-/internal/domain"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Claims struct {
	UserID          uuid.UUID `json:"user_id"`
	Email           string    `json:"email"`
	TenantID        uuid.UUID `json:"tenant_id"`
	TenantCode      string    `json:"tenant_code"`
	Roles           []string  `json:"roles"`
	Permissions     []string  `json:"permissions"`
	IsPlatformAdmin bool      `json:"is_platform_admin"`
	jwt.RegisteredClaims
}

type JWTService struct {
	secret        []byte
	accessExpiry  time.Duration
	refreshExpiry time.Duration
}

func NewJWTService(secret string, accessExpiry, refreshExpiry time.Duration) *JWTService {
	return &JWTService{
		secret:        []byte(secret),
		accessExpiry:  accessExpiry,
		refreshExpiry: refreshExpiry,
	}
}

func (s *JWTService) GenerateAccessToken(user *domain.User, tenant *domain.Tenant, roles, permissions []string) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:          user.ID,
		Email:           user.Email,
		TenantID:        tenant.ID,
		TenantCode:      tenant.Code,
		Roles:           roles,
		Permissions:     permissions,
		IsPlatformAdmin: user.IsPlatformAdmin,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.ID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.accessExpiry)),
			ID:        uuid.New().String(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString(s.secret)
	if err != nil {
		return "", fmt.Errorf("failed to sign access token: %w", err)
	}

	return signedToken, nil
}

func (s *JWTService) GenerateRefreshToken() (rawToken string, tokenHash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", fmt.Errorf("failed to generate random token: %w", err)
	}

	rawToken = hex.EncodeToString(b)
	tokenHash = s.HashRefreshToken(rawToken)
	return rawToken, tokenHash, nil
}

func (s *JWTService) HashRefreshToken(rawToken string) string {
	hash := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(hash[:])
}

func (s *JWTService) ValidateAccessToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.secret, nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, domain.ErrInvalidToken
		}
		return nil, domain.ErrUnauthorized
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, domain.ErrUnauthorized
	}

	return claims, nil
}

func (s *JWTService) RefreshExpiry() time.Duration {
	return s.refreshExpiry
}
