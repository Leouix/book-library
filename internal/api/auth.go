package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"book-library/internal/storage"
)

// UserStore is the interface for user storage operations.
type UserStore interface {
	CreateUser(ctx context.Context, username, passwordHash string) (storage.User, error)
	GetUserByUsername(ctx context.Context, username string) (storage.User, error)
}

// claims represents the JWT claims carried by auth tokens.
type claims struct {
	jwt.RegisteredClaims
	Username string `json:"username"`
}

type contextKey string

const userContextKey contextKey = "username"

// UsernameFromContext retrieves the authenticated username from context.
func UsernameFromContext(ctx context.Context) (string, bool) {
	s, ok := ctx.Value(userContextKey).(string)
	return s, ok
}

// AuthMiddleware returns an http.Handler that rejects requests without a valid
// Bearer JWT in the Authorization header.
func (h *Handler) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		if header == "" || !strings.HasPrefix(header, "Bearer ") {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing or malformed Authorization header"})
			return
		}

		tokenStr := strings.TrimPrefix(header, "Bearer ")

		var c claims
		token, err := jwt.ParseWithClaims(tokenStr, &c, func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return h.jwtSecret, nil
		})
		if err != nil || !token.Valid {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid or expired token"})
			return
		}

		ctx := context.WithValue(r.Context(), userContextKey, c.Username)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// ---------- request / response types ----------

type registerRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type authResponse struct {
	Token    string `json:"token,omitempty"`
	Username string `json:"username"`
}

// ---------- handlers ----------

// Register handles POST /register.
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {

	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	if req.Username == "" || req.Password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "username and password are required"})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("bcrypt error: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	user, err := h.userStore.CreateUser(r.Context(), req.Username, string(hash))
	if err != nil {
		log.Printf("CreateUser error: %v", err)
		writeJSON(w, http.StatusConflict, map[string]string{"error": "username already exists"})
		return
	}

	writeJSON(w, http.StatusCreated, authResponse{Username: user.Username})
}

// Login handles POST /login.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	if req.Username == "" || req.Password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "username and password are required"})
		return
	}

	user, err := h.userStore.GetUserByUsername(r.Context(), req.Username)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid username or password"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid username or password"})
		return
	}

	now := time.Now()
	c := claims{
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(24 * time.Hour)),
		},
		Username: req.Username,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	signed, err := token.SignedString(h.jwtSecret)
	if err != nil {
		log.Printf("JWT sign error: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, authResponse{Token: signed, Username: req.Username})
}
