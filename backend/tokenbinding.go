package backend

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/tink-crypto/tink-go/v2/hybrid"
	"github.com/tink-crypto/tink-go/v2/keyset"
)

const tokenBindingECDSAAliasPrefix = "auth_account:ecdsa_keypair:"

type tokenBindingSession struct {
	privateKeyset *keyset.Handle
}

type tokenBindingAliasKey struct {
	privatePKCS8 []byte
	publicSPKI   []byte
}

func newTokenBindingSession(alias string) (*tokenBindingSession, string, error) {
	bindingKey, err := parseTokenBindingECDSAAlias(alias)
	if err != nil {
		return nil, "", err
	}

	privateKey, err := parseTokenBindingPrivateKey(bindingKey.privatePKCS8)
	if err != nil {
		return nil, "", err
	}

	publicHash := sha256.Sum256(bindingKey.publicSPKI)
	issuer := base64.RawURLEncoding.EncodeToString(publicHash[:])

	privateHandle, publicKeyset, err := generateTokenBindingKeyset()
	if err != nil {
		return nil, "", err
	}

	assertionJWT, err := buildTokenBindingAssertionJWT(privateKey, issuer, base64.RawURLEncoding.EncodeToString(publicKeyset))
	if err != nil {
		return nil, "", err
	}

	return &tokenBindingSession{privateKeyset: privateHandle}, assertionJWT, nil
}

func (s *tokenBindingSession) decryptToken(encryptedToken string) (string, error) {
	if s == nil || s.privateKeyset == nil {
		return "", errors.New("token binding private keyset is unavailable")
	}

	ciphertext, err := decodeBase64URL(encryptedToken)
	if err != nil {
		return "", fmt.Errorf("failed to decode encrypted token: %w", err)
	}

	decryptor, err := hybrid.NewHybridDecrypt(s.privateKeyset)
	if err != nil {
		return "", fmt.Errorf("failed to create hybrid decrypt primitive: %w", err)
	}

	plaintext, err := decryptor.Decrypt(ciphertext, []byte{})
	if err != nil {
		return "", fmt.Errorf("failed to decrypt token: %w", err)
	}

	return string(plaintext), nil
}

func parseTokenBindingECDSAAlias(alias string) (*tokenBindingAliasKey, error) {
	if !strings.HasPrefix(alias, tokenBindingECDSAAliasPrefix) {
		return nil, fmt.Errorf("unsupported token binding alias type %q", aliasType(alias))
	}

	raw, err := decodeBase64URL(strings.TrimPrefix(alias, tokenBindingECDSAAliasPrefix))
	if err != nil {
		return nil, fmt.Errorf("failed to decode token binding alias: %w", err)
	}

	fields, err := parseLengthDelimitedProtoFields(raw)
	if err != nil {
		return nil, fmt.Errorf("failed to parse token binding alias: %w", err)
	}

	key := &tokenBindingAliasKey{
		privatePKCS8: firstBytesField(fields[1]),
		publicSPKI:   firstBytesField(fields[2]),
	}
	if len(key.privatePKCS8) == 0 || len(key.publicSPKI) == 0 {
		return nil, errors.New("token binding alias is missing private or public key material")
	}

	return key, nil
}

func parseTokenBindingPrivateKey(pkcs8 []byte) (*ecdsa.PrivateKey, error) {
	key, err := x509.ParsePKCS8PrivateKey(pkcs8)
	if err != nil {
		return nil, fmt.Errorf("failed to parse token binding private key: %w", err)
	}

	ecdsaKey, ok := key.(*ecdsa.PrivateKey)
	if !ok {
		return nil, errors.New("token binding private key is not ECDSA")
	}

	return ecdsaKey, nil
}

func generateTokenBindingKeyset() (*keyset.Handle, []byte, error) {
	manager := keyset.NewManager()
	keyID, err := manager.Add(hybrid.ECIESHKDFAES128GCMKeyTemplate())
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate token binding key: %w", err)
	}
	if err := manager.SetPrimary(keyID); err != nil {
		return nil, nil, fmt.Errorf("failed to set token binding primary key: %w", err)
	}

	privateHandle, err := manager.Handle()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create token binding keyset handle: %w", err)
	}

	publicHandle, err := privateHandle.Public()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create token binding public keyset: %w", err)
	}

	publicKeyset := &bytes.Buffer{}
	if err := publicHandle.WriteWithNoSecrets(keyset.NewBinaryWriter(publicKeyset)); err != nil {
		return nil, nil, fmt.Errorf("failed to serialize token binding public keyset: %w", err)
	}

	return privateHandle, publicKeyset.Bytes(), nil
}

func buildTokenBindingAssertionJWT(signingKey *ecdsa.PrivateKey, issuer string, publicKeysetB64 string) (string, error) {
	header := map[string]string{"alg": "ES256", "typ": "JWT"}
	payload := map[string]any{
		"namespace": "TokenBinding",
		"aud":       "https://accounts.google.com/accountmanager",
		"iss":       issuer,
		"iat":       time.Now().Unix(),
		"ephemeral_key": map[string]string{
			"kty":                     "type.googleapis.com/google.crypto.tink.EciesAeadHkdfPublicKey",
			"TinkKeysetPublicKeyInfo": publicKeysetB64,
		},
	}

	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	signingInput := base64.RawURLEncoding.EncodeToString(headerJSON) + "." + base64.RawURLEncoding.EncodeToString(payloadJSON)
	digest := sha256.Sum256([]byte(signingInput))
	r, s, err := ecdsa.Sign(rand.Reader, signingKey, digest[:])
	if err != nil {
		return "", fmt.Errorf("failed to sign assertion JWT: %w", err)
	}

	signature := fixedWidthP256Int(r)
	signature = append(signature, fixedWidthP256Int(s)...)
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func decryptTokenEncryptedResponse(parsed map[string]string, session *tokenBindingSession) error {
	if parsed["TokenEncrypted"] != "1" {
		return nil
	}
	if session == nil {
		return errors.New("auth response returned TokenEncrypted=1 but credential has no token_binding_alias")
	}

	encryptedToken := parsed["Auth"]
	if encryptedToken == "" {
		if parsed["it"] != "" {
			return errors.New("auth response returned encrypted it token; remove it_caveat_types to request encrypted Auth")
		}
		return errors.New("auth response returned TokenEncrypted=1 but no encrypted Auth field was present")
	}

	plaintext, err := session.decryptToken(encryptedToken)
	if err != nil {
		return err
	}

	parsed["Auth"] = plaintext
	return nil
}

func decodeBase64URL(s string) ([]byte, error) {
	if out, err := base64.RawURLEncoding.DecodeString(s); err == nil {
		return out, nil
	}
	return base64.URLEncoding.DecodeString(s)
}

func fixedWidthP256Int(x *big.Int) []byte {
	b := x.Bytes()
	if len(b) > 32 {
		return b[len(b)-32:]
	}
	out := make([]byte, 32)
	copy(out[32-len(b):], b)
	return out
}

func parseLengthDelimitedProtoFields(b []byte) (map[int][][]byte, error) {
	out := map[int][][]byte{}
	for offset := 0; offset < len(b); {
		key, n, ok := readProtoVarint(b[offset:])
		if !ok {
			return nil, fmt.Errorf("invalid protobuf key at offset %d", offset)
		}
		offset += n

		fieldNumber := int(key >> 3)
		wireType := int(key & 7)
		switch wireType {
		case 0:
			_, n, ok := readProtoVarint(b[offset:])
			if !ok {
				return nil, fmt.Errorf("invalid protobuf varint at field %d", fieldNumber)
			}
			offset += n
		case 1:
			offset += 8
		case 2:
			fieldLen, n, ok := readProtoVarint(b[offset:])
			if !ok {
				return nil, fmt.Errorf("invalid protobuf length at field %d", fieldNumber)
			}
			offset += n
			if fieldLen > uint64(len(b)-offset) {
				return nil, fmt.Errorf("protobuf field %d overruns message", fieldNumber)
			}
			out[fieldNumber] = append(out[fieldNumber], append([]byte(nil), b[offset:offset+int(fieldLen)]...))
			offset += int(fieldLen)
		case 5:
			offset += 4
		default:
			return nil, fmt.Errorf("unsupported protobuf wire type %d", wireType)
		}

		if offset > len(b) {
			return nil, errors.New("protobuf field overruns message")
		}
	}
	return out, nil
}

func readProtoVarint(b []byte) (uint64, int, bool) {
	var value uint64
	for i, c := range b {
		if i == 10 {
			return 0, 0, false
		}
		value |= uint64(c&0x7f) << (7 * i)
		if c&0x80 == 0 {
			return value, i + 1, true
		}
	}
	return 0, 0, false
}

func firstBytesField(fields [][]byte) []byte {
	if len(fields) == 0 {
		return nil
	}
	return fields[0]
}

func aliasType(alias string) string {
	if idx := strings.Index(alias, ":"); idx >= 0 {
		parts := strings.Split(alias, ":")
		if len(parts) >= 2 {
			return strings.Join(parts[:min(len(parts), 3)], ":")
		}
	}
	return alias
}
