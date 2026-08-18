package credential

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"bac-nexus/internal/strictjson"

	"golang.org/x/crypto/argon2"
)

const (
	Version          = 1
	DefaultTime      = uint32(3)
	DefaultMemoryKiB = uint32(64 * 1024)
	DefaultThreads   = uint8(4)
	DefaultKeyLen    = uint32(32)
	MaxVaultBytes    = 16 * 1024
	maxSecretBytes   = 4096
)

var ErrNotFound = errors.New("credential vault not found")

type Parameters struct {
	Time      uint32
	MemoryKiB uint32
	Threads   uint8
	KeyLen    uint32
}

var ProductionParameters = Parameters{Time: DefaultTime, MemoryKiB: DefaultMemoryKiB, Threads: DefaultThreads, KeyLen: DefaultKeyLen}

type parameterPolicy struct{ approved []Parameters }

var productionPolicy = parameterPolicy{approved: []Parameters{ProductionParameters}}

type SetResult struct {
	Path           string
	Committed      bool
	CleanupWarning error
}

type envelope struct {
	Version      int    `json:"version"`
	KDFTime      uint32 `json:"kdfTime"`
	KDFMemoryKiB uint32 `json:"kdfMemoryKiB"`
	KDFThreads   uint8  `json:"kdfThreads"`
	KDFKeyLen    uint32 `json:"kdfKeyLen"`
	Salt         string `json:"salt"`
	Nonce        string `json:"nonce"`
	Ciphertext   string `json:"ciphertext"`
}

type Store struct {
	Root       string
	Random     io.Reader
	Parameters Parameters
	policy     *parameterPolicy
	files      *fileOperations
}

type fileOperations struct {
	link   func(string, string) error
	remove func(string) error
	stat   func(string) (os.FileInfo, error)
}

var operatingSystemFiles = fileOperations{link: os.Link, remove: os.Remove, stat: os.Stat}

func DefaultRoot() (string, error) {
	root, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "BAC Nexus", "credentials"), nil
}

func (s Store) Set(profile string, password, master []byte, replace bool) (SetResult, error) {
	if err := validateProfileName(profile); err != nil {
		return SetResult{}, err
	}
	if len(password) == 0 || len(password) > maxSecretBytes {
		return SetResult{}, errors.New("IBM i password length is invalid")
	}
	if len(master) == 0 || len(master) > maxSecretBytes {
		return SetResult{}, errors.New("vault master passphrase length is invalid")
	}
	parameters := s.Parameters
	if parameters == (Parameters{}) {
		parameters = ProductionParameters
	}
	if err := s.parameterPolicy().validate(parameters); err != nil {
		return SetResult{}, err
	}
	random := s.Random
	if random == nil {
		random = rand.Reader
	}
	salt := make([]byte, 16)
	if _, err := io.ReadFull(random, salt); err != nil {
		return SetResult{}, errors.New("generate vault salt")
	}
	key := argon2.IDKey(master, salt, parameters.Time, parameters.MemoryKiB, parameters.Threads, parameters.KeyLen)
	defer Zero(key)
	block, err := aes.NewCipher(key)
	if err != nil {
		return SetResult{}, errors.New("initialize vault cipher")
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return SetResult{}, errors.New("initialize vault authentication")
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(random, nonce); err != nil {
		return SetResult{}, errors.New("generate vault nonce")
	}
	ciphertext := gcm.Seal(nil, nonce, password, additionalData(profile))
	envelope := envelope{
		Version: Version, KDFTime: parameters.Time, KDFMemoryKiB: parameters.MemoryKiB, KDFThreads: parameters.Threads, KDFKeyLen: parameters.KeyLen,
		Salt: encode(salt), Nonce: encode(nonce), Ciphertext: encode(ciphertext),
	}
	data, err := json.Marshal(envelope)
	if err != nil {
		return SetResult{}, errors.New("encode credential vault")
	}
	data = append(data, '\n')
	return s.publish(profile, data, replace)
}

func (s Store) Get(profile string, master []byte) ([]byte, error) {
	if err := validateProfileName(profile); err != nil {
		return nil, err
	}
	if len(master) == 0 || len(master) > maxSecretBytes {
		return nil, errors.New("vault master passphrase length is invalid")
	}
	if err := s.recover(profile); err != nil {
		return nil, err
	}
	file, err := os.Open(s.path(profile))
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, MaxVaultBytes+1))
	if err != nil {
		return nil, errors.New("read credential vault")
	}
	if len(data) > MaxVaultBytes {
		return nil, errors.New("credential vault exceeds byte limit")
	}
	envelope, salt, nonce, ciphertext, err := decodeEnvelope(data, s.parameterPolicy())
	if err != nil {
		return nil, err
	}
	parameters := Parameters{Time: envelope.KDFTime, MemoryKiB: envelope.KDFMemoryKiB, Threads: envelope.KDFThreads, KeyLen: envelope.KDFKeyLen}
	key := argon2.IDKey(master, salt, parameters.Time, parameters.MemoryKiB, parameters.Threads, parameters.KeyLen)
	defer Zero(key)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, errors.New("initialize vault cipher")
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, errors.New("initialize vault authentication")
	}
	password, err := gcm.Open(nil, nonce, ciphertext, additionalData(profile))
	if err != nil {
		return nil, errors.New("credential vault authentication failed")
	}
	if len(password) == 0 || len(password) > maxSecretBytes {
		Zero(password)
		return nil, errors.New("decrypted credential length is invalid")
	}
	return password, nil
}

func (s Store) Status(profile string) (bool, error) {
	if err := validateProfileName(profile); err != nil {
		return false, err
	}
	if err := s.recover(profile); err != nil {
		return false, err
	}
	_, err := os.Stat(s.path(profile))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

func (s Store) Delete(profile string) (bool, error) {
	if err := validateProfileName(profile); err != nil {
		return false, err
	}
	path := s.path(profile)
	err := os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		_ = os.Remove(path + ".rollback")
		return false, nil
	}
	if err != nil {
		return false, err
	}
	_ = os.Remove(path + ".rollback")
	return true, nil
}

func (s Store) publish(profile string, data []byte, replace bool) (SetResult, error) {
	if s.Root == "" {
		return SetResult{}, errors.New("credential store root is required")
	}
	if err := os.MkdirAll(s.Root, 0o700); err != nil {
		return SetResult{}, err
	}
	if err := s.recover(profile); err != nil {
		return SetResult{}, err
	}
	path := s.path(profile)
	temp, err := os.CreateTemp(s.Root, ".vault-*.tmp")
	if err != nil {
		return SetResult{}, err
	}
	tempPath := temp.Name()
	defer func() { _ = temp.Close(); _ = os.Remove(tempPath) }()
	if err := temp.Chmod(0o600); err != nil {
		return SetResult{}, err
	}
	if _, err := temp.Write(data); err != nil {
		return SetResult{}, err
	}
	if err := temp.Sync(); err != nil {
		return SetResult{}, err
	}
	if err := temp.Close(); err != nil {
		return SetResult{}, err
	}
	if !replace {
		if err := s.fileOps().link(tempPath, path); err != nil {
			if errors.Is(err, os.ErrExist) {
				return SetResult{}, fmt.Errorf("credential vault for %q already exists: %w", profile, os.ErrExist)
			}
			return SetResult{}, err
		}
		return SetResult{Path: path, Committed: true}, nil
	}
	ops := s.fileOps()
	if _, err := ops.stat(path); errors.Is(err, os.ErrNotExist) {
		return SetResult{}, fmt.Errorf("credential vault for %q does not exist; omit -replace to create it", profile)
	} else if err != nil {
		return SetResult{}, err
	}
	backup := path + ".rollback"
	if err := ops.link(path, backup); err != nil {
		return SetResult{}, fmt.Errorf("prepare credential rotation rollback: %w", err)
	}
	if err := ops.remove(path); err != nil {
		cleanupErr := ops.remove(backup)
		return SetResult{}, errors.Join(err, cleanupErr)
	}
	if err := ops.link(tempPath, path); err != nil {
		rollbackErr := ops.link(backup, path)
		return SetResult{}, errors.Join(fmt.Errorf("publish rotated credential vault: %w", err), rollbackErr)
	}
	result := SetResult{Path: path, Committed: true}
	if err := ops.remove(backup); err != nil {
		result.CleanupWarning = fmt.Errorf("credential rotated; rollback cleanup pending: %w", err)
	}
	return result, nil
}

func (s Store) recover(profile string) error {
	path := s.path(profile)
	backup := path + ".rollback"
	ops := s.fileOps()
	if _, err := ops.stat(path); err == nil {
		if err := ops.remove(backup); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("clean credential rotation rollback: %w", err)
		}
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if _, err := ops.stat(backup); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	if err := ops.link(backup, path); err != nil {
		return fmt.Errorf("recover interrupted credential rotation: %w", err)
	}
	return ops.remove(backup)
}

func decodeEnvelope(data []byte, policy *parameterPolicy) (envelope, []byte, []byte, []byte, error) {
	var value envelope
	if err := strictjson.ValidateObjectKeys(data, "version", "kdfTime", "kdfMemoryKiB", "kdfThreads", "kdfKeyLen", "salt", "nonce", "ciphertext"); err != nil {
		return value, nil, nil, nil, errors.New("decode credential vault")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return value, nil, nil, nil, errors.New("decode credential vault")
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return value, nil, nil, nil, errors.New("credential vault contains trailing data")
	}
	if value.Version != Version {
		return value, nil, nil, nil, errors.New("unsupported credential vault version")
	}
	if err := policy.validate(Parameters{Time: value.KDFTime, MemoryKiB: value.KDFMemoryKiB, Threads: value.KDFThreads, KeyLen: value.KDFKeyLen}); err != nil {
		return value, nil, nil, nil, err
	}
	salt, err := decode(value.Salt, 16, 16)
	if err != nil {
		return value, nil, nil, nil, errors.New("invalid credential vault salt")
	}
	nonce, err := decode(value.Nonce, 12, 12)
	if err != nil {
		return value, nil, nil, nil, errors.New("invalid credential vault nonce")
	}
	ciphertext, err := decode(value.Ciphertext, 17, maxSecretBytes+16)
	if err != nil {
		return value, nil, nil, nil, errors.New("invalid credential vault ciphertext")
	}
	return value, salt, nonce, ciphertext, nil
}

func (p parameterPolicy) validate(parameters Parameters) error {
	for _, approved := range p.approved {
		if parameters == approved {
			return nil
		}
	}
	return errors.New("credential vault KDF parameters are not approved for this version")
}

func (s Store) parameterPolicy() *parameterPolicy {
	if s.policy != nil {
		return s.policy
	}
	return &productionPolicy
}

func (s Store) fileOps() *fileOperations {
	if s.files != nil {
		return s.files
	}
	return &operatingSystemFiles
}

func validateProfileName(name string) error {
	if len(name) < 1 || len(name) > 64 || strings.ContainsAny(name, `/\\`) {
		return errors.New("invalid credential profile name")
	}
	for i, r := range name {
		if !((r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || (i > 0 && strings.ContainsRune("._-", r))) {
			return errors.New("invalid credential profile name")
		}
	}
	return nil
}

func (s Store) path(profile string) string { return filepath.Join(s.Root, profile+".vault") }
func additionalData(profile string) []byte {
	return []byte("BAC Nexus/catalogspike/credential-vault/v1/" + profile)
}
func encode(value []byte) string { return base64.RawStdEncoding.EncodeToString(value) }

func decode(value string, minimum, maximum int) ([]byte, error) {
	if len(value) > base64.RawStdEncoding.EncodedLen(maximum) {
		return nil, errors.New("encoded field too large")
	}
	if strings.IndexFunc(value, func(r rune) bool { return r == '\r' || r == '\n' || r == '\t' || r == ' ' }) >= 0 {
		return nil, errors.New("invalid encoded field")
	}
	decoded, err := base64.RawStdEncoding.DecodeString(value)
	if err != nil || len(decoded) < minimum || len(decoded) > maximum || base64.RawStdEncoding.EncodeToString(decoded) != value {
		return nil, errors.New("invalid encoded field")
	}
	return decoded, nil
}

func Zero(value []byte) {
	for i := range value {
		value[i] = 0
	}
}
