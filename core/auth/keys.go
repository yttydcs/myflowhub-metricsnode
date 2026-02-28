package auth

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

type nodeKeys struct {
	PrivKey string `json:"privkey"`
	PubKey  string `json:"pubkey"`
}

type KeyStore struct {
	mu   sync.Mutex
	path string

	priv *ecdsa.PrivateKey
	pub  string
}

func NewKeyStore(path string) *KeyStore {
	return &KeyStore{path: filepath.Clean(path)}
}

func (s *KeyStore) Ensure() (string, error) {
	if s == nil {
		return "", errors.New("keystore not initialized")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.priv != nil && strings.TrimSpace(s.pub) != "" {
		return s.pub, nil
	}
	priv, pub, err := loadOrCreateNodeKeys(s.path)
	if err != nil {
		return "", err
	}
	s.priv = priv
	s.pub = pub
	return pub, nil
}

func (s *KeyStore) SignLogin(deviceID string, nodeID uint32, ts int64, nonce string) (string, error) {
	if s == nil {
		return "", errors.New("keystore not initialized")
	}
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return "", errors.New("device_id is required")
	}
	if nodeID == 0 {
		return "", errors.New("node_id is required")
	}
	nonce = strings.TrimSpace(nonce)
	if nonce == "" {
		return "", errors.New("nonce is required")
	}

	if _, err := s.Ensure(); err != nil {
		return "", err
	}

	s.mu.Lock()
	priv := s.priv
	s.mu.Unlock()
	if priv == nil {
		return "", errors.New("private key invalid")
	}
	return signLogin(priv, deviceID, nodeID, ts, nonce)
}

func loadOrCreateNodeKeys(path string) (*ecdsa.PrivateKey, string, error) {
	if priv, pub, err := readNodeKeys(path); err == nil && priv != nil && strings.TrimSpace(pub) != "" {
		return priv, pub, nil
	}
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, "", err
	}
	privDER, _ := x509.MarshalECPrivateKey(priv)
	pubDER, _ := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	privB64 := base64.StdEncoding.EncodeToString(privDER)
	pubB64 := base64.StdEncoding.EncodeToString(pubDER)
	_ = writeNodeKeys(path, nodeKeys{PrivKey: privB64, PubKey: pubB64})
	return priv, pubB64, nil
}

func readNodeKeys(path string) (*ecdsa.PrivateKey, string, error) {
	path = filepath.Clean(path)
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return nil, "", err
	}
	var k nodeKeys
	if err := json.Unmarshal(data, &k); err != nil {
		return nil, "", err
	}
	priv, err := parsePrivKey(k.PrivKey)
	if err != nil {
		return nil, "", err
	}
	if strings.TrimSpace(k.PubKey) == "" {
		return nil, "", errors.New("pubkey empty")
	}
	return priv, k.PubKey, nil
}

func writeNodeKeys(path string, k nodeKeys) error {
	path = filepath.Clean(path)
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	data, _ := json.MarshalIndent(k, "", "  ")
	return os.WriteFile(path, data, 0o600)
}

func parsePrivKey(b64 string) (*ecdsa.PrivateKey, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(b64))
	if err != nil {
		return nil, err
	}
	priv, err := x509.ParseECPrivateKey(raw)
	if err != nil {
		return nil, err
	}
	if priv == nil || priv.Curve != elliptic.P256() {
		return nil, errors.New("private key not p256")
	}
	return priv, nil
}

func loginSignBytes(deviceID string, nodeID uint32, ts int64, nonce string) []byte {
	sb := strings.Builder{}
	sb.WriteString("login\n")
	sb.WriteString(strings.TrimSpace(deviceID))
	sb.WriteString("\n")
	sb.WriteString(uintToString(nodeID))
	sb.WriteString("\n")
	sb.WriteString(strconv.FormatInt(ts, 10))
	sb.WriteString("\n")
	sb.WriteString(strings.TrimSpace(nonce))
	return []byte(sb.String())
}

func uintToString(v uint32) string {
	if v == 0 {
		return "0"
	}
	return strconv.FormatUint(uint64(v), 10)
}

func signLogin(priv *ecdsa.PrivateKey, deviceID string, nodeID uint32, ts int64, nonce string) (string, error) {
	if priv == nil {
		return "", errors.New("private key nil")
	}
	msg := loginSignBytes(deviceID, nodeID, ts, nonce)
	hashed := sha256.Sum256(msg)
	sig, err := ecdsa.SignASN1(rand.Reader, priv, hashed[:])
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(sig), nil
}

func GenerateNonce(n int) string {
	if n <= 0 {
		n = 12
	}
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return ""
	}
	return hex.EncodeToString(buf)
}

