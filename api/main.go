package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Server struct {
	db *pgxpool.Pool
}

type RegisterRequest struct {
	Email     string `json:"email"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
}

type RecognizeRequest struct {
	Email string `json:"email"`
}

type VerifyOTPRequest struct {
	Email string `json:"email"`
	Code  string `json:"code"`
}

type CheckoutRequest struct {
	Email           string `json:"email"`
	PhoneNumber     string `json:"phoneNumber"`
	ShippingAddress string `json:"shippingAddress"`
}

type User struct {
	ID        int64  `json:"id"`
	Email     string `json:"email"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
}

var emailPattern = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

func main() {
	ctx := context.Background()
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	db, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := db.Ping(ctx); err != nil {
		log.Fatal(err)
	}

	server := &Server{db: db}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", server.health)
	mux.HandleFunc("/api/auth/register", server.register)
	mux.HandleFunc("/api/auth/recognize", server.recognize)
	mux.HandleFunc("/api/auth/verify", server.verifyOTP)
	mux.HandleFunc("/api/auth/me", server.me)
	mux.HandleFunc("/api/checkout", server.checkout)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("API listening on :%s", port)
	if err := http.ListenAndServe(":"+port, withCORS(mux)); err != nil {
		log.Fatal(err)
	}
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := os.Getenv("FRONTEND_URL")
		if origin == "" {
			origin = "http://localhost:5173"
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var input RegisterRequest
	if !decodeJSON(w, r, &input) {
		return
	}

	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	input.FirstName = strings.TrimSpace(input.FirstName)
	input.LastName = strings.TrimSpace(input.LastName)

	if !emailPattern.MatchString(input.Email) || input.FirstName == "" || input.LastName == "" {
		writeError(w, http.StatusBadRequest, "enter a valid email, first name, and last name")
		return
	}

	code, err := createOTP()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not generate login code")
		return
	}

	var userID int64
	err = s.db.QueryRow(r.Context(), `
		INSERT INTO users (email, first_name, last_name, login_code)
		VALUES ($1, $2, $3, $4)
		RETURNING id`, input.Email, input.FirstName, input.LastName, code).Scan(&userID)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			writeError(w, http.StatusConflict, "an account with this email already exists")
			return
		}
		log.Printf("register: %v", err)
		writeError(w, http.StatusInternalServerError, "could not create account")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"message": "registration successful",
		"user":    User{ID: userID, Email: input.Email, FirstName: input.FirstName, LastName: input.LastName},
		"code":    code,
	})
}

func (s *Server) recognize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var input RecognizeRequest
	if !decodeJSON(w, r, &input) {
		return
	}
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))

	if !emailPattern.MatchString(input.Email) {
		writeJSON(w, http.StatusOK, map[string]bool{"registered": false})
		return
	}

	var user User
	err := s.db.QueryRow(r.Context(), `
		SELECT id, email, first_name, last_name
		FROM users WHERE email = $1`, input.Email).Scan(&user.ID, &user.Email, &user.FirstName, &user.LastName)

	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusOK, map[string]bool{"registered": false})
		return
	}
	if err != nil {
		log.Printf("recognize: %v", err)
		writeError(w, http.StatusInternalServerError, "recognition check failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"registered": true,
		"firstName":  user.FirstName,
		"lastName":   user.LastName,
	})
}

func (s *Server) verifyOTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var input VerifyOTPRequest
	if !decodeJSON(w, r, &input) {
		return
	}
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	input.Code = strings.TrimSpace(input.Code)

	if len(input.Code) != 6 {
		writeError(w, http.StatusBadRequest, "enter the 6-digit code")
		return
	}

	var user User
	var storedCode string
	err := s.db.QueryRow(r.Context(), `
		SELECT id, email, first_name, last_name, login_code
		FROM users WHERE email = $1`, input.Email).
		Scan(&user.ID, &user.Email, &user.FirstName, &user.LastName, &storedCode)

	if err == pgx.ErrNoRows {
		writeError(w, http.StatusUnauthorized, "invalid code")
		return
	}
	if err != nil {
		log.Printf("verify lookup: %v", err)
		writeError(w, http.StatusInternalServerError, "could not verify code")
		return
	}

	if input.Code != storedCode {
		writeError(w, http.StatusUnauthorized, "incorrect code")
		return
	}

	sessionToken, err := randomToken(32)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create session")
		return
	}

	_, err = s.db.Exec(r.Context(), `
		INSERT INTO sessions (token, user_id, expires_at)
		VALUES ($1, $2, NOW() + INTERVAL '7 days')`, sessionToken, user.ID)
	if err != nil {
		log.Printf("session create: %v", err)
		writeError(w, http.StatusInternalServerError, "could not create session")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "otp_session",
		Value:    sessionToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   os.Getenv("COOKIE_SECURE") == "true",
		SameSite: cookieSameSite(),
		MaxAge:   7 * 24 * 60 * 60,
	})

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message": "login successful",
		"user":    user,
	})
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	user, ok := s.currentUser(r.Context(), r)
	if !ok {
		writeJSON(w, http.StatusOK, map[string]interface{}{"loggedIn": false})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"loggedIn": true, "user": user})
}

func (s *Server) checkout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var input CheckoutRequest
	if !decodeJSON(w, r, &input) {
		return
	}
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	input.PhoneNumber = strings.TrimSpace(input.PhoneNumber)
	input.ShippingAddress = strings.TrimSpace(input.ShippingAddress)

	if !emailPattern.MatchString(input.Email) || input.PhoneNumber == "" || input.ShippingAddress == "" {
		writeError(w, http.StatusBadRequest, "please complete the checkout form")
		return
	}

	var userID *int64
	if user, ok := s.currentUser(r.Context(), r); ok {
		userID = &user.ID
	}

	_, err := s.db.Exec(r.Context(), `
		INSERT INTO checkout_orders (user_id, email, phone_number, shipping_address)
		VALUES ($1, $2, $3, $4)`, userID, input.Email, input.PhoneNumber, input.ShippingAddress)
	if err != nil {
		log.Printf("checkout: %v", err)
		writeError(w, http.StatusInternalServerError, "could not save checkout details")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"message": "checkout details saved"})
}

func (s *Server) currentUser(ctx context.Context, r *http.Request) (User, bool) {
	cookie, err := r.Cookie("otp_session")
	if err != nil || cookie.Value == "" {
		return User{}, false
	}

	var user User
	err = s.db.QueryRow(ctx, `
		SELECT u.id, u.email, u.first_name, u.last_name
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.token = $1 AND s.expires_at > NOW()`, cookie.Value).
		Scan(&user.ID, &user.Email, &user.FirstName, &user.LastName)
	if err != nil {
		return User{}, false
	}
	return user, true
}

func createOTP() (string, error) {
	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	value := uint32(buf[0])<<24 | uint32(buf[1])<<16 | uint32(buf[2])<<8 | uint32(buf[3])
	return fmt.Sprintf("%06d", value%1000000), nil
}

func randomToken(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target interface{}) bool {
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(target); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, value interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func cookieSameSite() http.SameSite {
	if os.Getenv("COOKIE_SECURE") == "true" {
		return http.SameSiteNoneMode
	}
	return http.SameSiteLaxMode
}
