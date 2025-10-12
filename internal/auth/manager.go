package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Manager handles email challenges and session token issuance.
type Manager struct {
	secret     []byte
	mu         sync.Mutex
	challenges map[string]challenge
	ttl        time.Duration
}

type challenge struct {
	email   string
	code    string
	expires time.Time
}

// NewManager creates a Manager with the provided secret.
func NewManager(secret string) *Manager {
	if secret == "" {
		panic("auth manager requires non-empty secret")
	}
	return &Manager{
		secret:     []byte(secret),
		challenges: make(map[string]challenge),
		ttl:        10 * time.Minute,
	}
}

// CreateChallenge registers a verification code for the email.
func (m *Manager) CreateChallenge(email string) (challengeID, code string, expires time.Time, err error) {
	if email == "" {
		return "", "", time.Time{}, errors.New("email required")
	}
	id, err := randomID()
	if err != nil {
		return "", "", time.Time{}, err
	}
	code, err = randomCode()
	if err != nil {
		return "", "", time.Time{}, err
	}
	expires = time.Now().Add(m.ttl)
	m.mu.Lock()
	m.challenges[id] = challenge{email: email, code: code, expires: expires}
	m.mu.Unlock()
	return id, code, expires, nil
}

// VerifyChallenge validates the code and returns the associated email.
func (m *Manager) VerifyChallenge(challengeID, code string) (string, error) {
	m.mu.Lock()
	c, ok := m.challenges[challengeID]
	if ok && time.Now().After(c.expires) {
		ok = false
		delete(m.challenges, challengeID)
	}
	if !ok {
		m.mu.Unlock()
		return "", errors.New("challenge not found or expired")
	}
	if c.code != code {
		m.mu.Unlock()
		return "", errors.New("invalid verification code")
	}
	delete(m.challenges, challengeID)
	m.mu.Unlock()
	return c.email, nil
}

// IssueToken issues a signed session token for the email.
func (m *Manager) IssueToken(email string, ttl time.Duration) (string, error) {
	if ttl == 0 {
		ttl = 24 * time.Hour
	}
	expires := time.Now().Add(ttl).Unix()
	payload := fmt.Sprintf("%s|%d", email, expires)
	sig := m.sign([]byte(payload))
	token := fmt.Sprintf("%s.%s", base64.RawURLEncoding.EncodeToString([]byte(payload)), base64.RawURLEncoding.EncodeToString(sig))
	return token, nil
}

// ValidateToken validates and returns the embedded email.
func (m *Manager) ValidateToken(token string) (string, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return "", errors.New("invalid token format")
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", errors.New("invalid token payload")
	}
	sigBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", errors.New("invalid token signature")
	}
	if !hmac.Equal(sigBytes, m.sign(payloadBytes)) {
		return "", errors.New("signature mismatch")
	}
	payload := string(payloadBytes)
	sep := strings.LastIndex(payload, "|")
	if sep == -1 {
		return "", errors.New("invalid payload")
	}
	email := payload[:sep]
	expiry, err := strconv.ParseInt(payload[sep+1:], 10, 64)
	if err != nil {
		return "", errors.New("invalid expiry")
	}
	if time.Now().Unix() > expiry {
		return "", errors.New("token expired")
	}
	return email, nil
}

func (m *Manager) sign(payload []byte) []byte {
	h := hmac.New(sha256.New, m.secret)
	h.Write(payload)
	return h.Sum(nil)
}

func randomID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func randomCode() (string, error) {
	var b [3]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	value := int(b[0])<<16 | int(b[1])<<8 | int(b[2])
	return fmt.Sprintf("%06d", value%1000000), nil
}
