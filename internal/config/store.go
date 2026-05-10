package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db  *sql.DB
	key []byte
}

func deriveKey(password string) []byte {
	h := sha256.Sum256([]byte("trout-enc:v1:" + password))
	return h[:]
}

func encrypt(plaintext []byte, key []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("new gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("nonce: %w", err)
	}
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func decrypt(ciphertext string, key []byte) (string, error) {
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("decode: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("new gcm: %w", err)
	}
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}
	plaintext, err := gcm.Open(nil, data[:nonceSize], data[nonceSize:], nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}
	return string(plaintext), nil
}

func Open(path string, password string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, fmt.Errorf("enable wal: %w", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		return nil, fmt.Errorf("enable fk: %w", err)
	}
	s := &Store{db: db, key: deriveKey(password)}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS challenges (
		id            TEXT PRIMARY KEY,
		name          TEXT NOT NULL,
		description   TEXT DEFAULT '',
		base_flag     TEXT NOT NULL,
		leet_rules    TEXT DEFAULT '{}',
		questions     TEXT NOT NULL DEFAULT '[]',
		netcat_port   INTEGER UNIQUE,
		enabled       INTEGER DEFAULT 1,
		created_at    TEXT DEFAULT (datetime('now')),
		updated_at    TEXT DEFAULT (datetime('now'))
	);
	CREATE TABLE IF NOT EXISTS ctfd_config (
		id                INTEGER PRIMARY KEY DEFAULT 1,
		url               TEXT NOT NULL DEFAULT '',
		api_key           TEXT NOT NULL DEFAULT '',
		webhook_secret    TEXT DEFAULT '',
		detected_edition  TEXT DEFAULT 'unknown',
		plugin_installed  INTEGER DEFAULT 0,
		poll_interval_sec INTEGER DEFAULT 30,
		updated_at        TEXT DEFAULT (datetime('now'))
	);
	CREATE TABLE IF NOT EXISTS discord_webhooks (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		url         TEXT NOT NULL,
		event_type  TEXT NOT NULL,
		enabled     INTEGER DEFAULT 1,
		created_at  TEXT DEFAULT (datetime('now'))
	);
	CREATE TABLE IF NOT EXISTS event_log (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		event_type  TEXT NOT NULL,
		source      TEXT NOT NULL DEFAULT '',
		payload     TEXT DEFAULT '{}',
		created_at  TEXT DEFAULT (datetime('now'))
	);
	`
	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("exec schema: %w", err)
	}
	return nil
}

func (s *Store) ListChallenges() ([]Challenge, error) {
	rows, err := s.db.Query(`
		SELECT id, name, description, base_flag, leet_rules, questions,
		       netcat_port, enabled, created_at, updated_at
		FROM challenges ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("list challenges: %w", err)
	}
	defer rows.Close()

	var out []Challenge
	for rows.Next() {
		var c Challenge
		var enabled int
		var created, updated string
		if err := rows.Scan(&c.ID, &c.Name, &c.Description, &c.BaseFlag,
			&c.LeetRules, &c.Questions, &c.NetcatPort, &enabled,
			&created, &updated); err != nil {
			return nil, fmt.Errorf("scan challenge: %w", err)
		}
		c.Enabled = enabled != 0
		c.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", created)
		c.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updated)
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) GetChallenge(id string) (*Challenge, error) {
	row := s.db.QueryRow(`
		SELECT id, name, description, base_flag, leet_rules, questions,
		       netcat_port, enabled, created_at, updated_at
		FROM challenges WHERE id = ?
	`, id)
	var c Challenge
	var enabled int
	var created, updated string
	if err := row.Scan(&c.ID, &c.Name, &c.Description, &c.BaseFlag,
		&c.LeetRules, &c.Questions, &c.NetcatPort, &enabled,
		&created, &updated); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get challenge: %w", err)
	}
	c.Enabled = enabled != 0
	c.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", created)
	c.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updated)
	return &c, nil
}

func (s *Store) UpsertChallenge(c Challenge) error {
	_, err := s.db.Exec(`
		INSERT INTO challenges (id, name, description, base_flag, leet_rules, questions, netcat_port, enabled, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, datetime('now'))
		ON CONFLICT(id) DO UPDATE SET
			name=excluded.name, description=excluded.description,
			base_flag=excluded.base_flag, leet_rules=excluded.leet_rules,
			questions=excluded.questions, netcat_port=excluded.netcat_port,
			enabled=excluded.enabled, updated_at=excluded.updated_at
	`, c.ID, c.Name, c.Description, c.BaseFlag, c.LeetRules,
		c.Questions, c.NetcatPort, boolInt(c.Enabled))
	if err != nil {
		return fmt.Errorf("upsert challenge: %w", err)
	}
	return nil
}

func (s *Store) DeleteChallenge(id string) error {
	_, err := s.db.Exec("DELETE FROM challenges WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete challenge: %w", err)
	}
	return nil
}

func (s *Store) GetCTFdConfig() (*CTFdConfig, error) {
	row := s.db.QueryRow(`
		SELECT id, url, api_key, webhook_secret, detected_edition,
		       plugin_installed, poll_interval_sec, updated_at
		FROM ctfd_config WHERE id = 1
	`)
	var cfg CTFdConfig
	var installed int
	var updated string
	var encAPIKey, encWebhookSecret string
	if err := row.Scan(&cfg.ID, &cfg.URL, &encAPIKey, &encWebhookSecret,
		&cfg.DetectedEdition, &installed, &cfg.PollIntervalSec,
		&updated); err != nil {
		if err == sql.ErrNoRows {
			return &CTFdConfig{
				ID:              1,
				DetectedEdition: "unknown",
				PollIntervalSec: 30,
			}, nil
		}
		return nil, fmt.Errorf("get ctfd config: %w", err)
	}
	if encAPIKey != "" {
		dec, err := decrypt(encAPIKey, s.key)
		if err != nil {
			return nil, fmt.Errorf("decrypt api key: %w", err)
		}
		cfg.APIKey = dec
	}
	if encWebhookSecret != "" {
		dec, err := decrypt(encWebhookSecret, s.key)
		if err != nil {
			return nil, fmt.Errorf("decrypt webhook secret: %w", err)
		}
		cfg.WebhookSecret = dec
	}
	cfg.PluginInstalled = installed != 0
	cfg.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updated)
	return &cfg, nil
}

func (s *Store) SaveCTFdConfig(cfg CTFdConfig) error {
	encAPIKey := cfg.APIKey
	if encAPIKey != "" {
		enc, err := encrypt([]byte(encAPIKey), s.key)
		if err != nil {
			return fmt.Errorf("encrypt api key: %w", err)
		}
		encAPIKey = enc
	}
	encWebhookSecret := cfg.WebhookSecret
	if encWebhookSecret != "" {
		enc, err := encrypt([]byte(encWebhookSecret), s.key)
		if err != nil {
			return fmt.Errorf("encrypt webhook secret: %w", err)
		}
		encWebhookSecret = enc
	}
	_, err := s.db.Exec(`
		INSERT INTO ctfd_config (id, url, api_key, webhook_secret, detected_edition,
		                         plugin_installed, poll_interval_sec, updated_at)
		VALUES (1, ?, ?, ?, ?, ?, ?, datetime('now'))
		ON CONFLICT(id) DO UPDATE SET
			url=excluded.url, api_key=excluded.api_key,
			webhook_secret=excluded.webhook_secret,
			detected_edition=excluded.detected_edition,
			plugin_installed=excluded.plugin_installed,
			poll_interval_sec=excluded.poll_interval_sec,
			updated_at=excluded.updated_at
	`, cfg.URL, encAPIKey, encWebhookSecret, cfg.DetectedEdition,
		boolInt(cfg.PluginInstalled), cfg.PollIntervalSec)
	if err != nil {
		return fmt.Errorf("save ctfd config: %w", err)
	}
	return nil
}

func (s *Store) ListDiscordWebhooks() ([]DiscordWebhook, error) {
	rows, err := s.db.Query(`
		SELECT id, url, event_type, enabled, created_at
		FROM discord_webhooks ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("list webhooks: %w", err)
	}
	defer rows.Close()

	var out []DiscordWebhook
	for rows.Next() {
		var h DiscordWebhook
		var enabled int
		var created string
		if err := rows.Scan(&h.ID, &h.URL, &h.EventType, &enabled, &created); err != nil {
			return nil, fmt.Errorf("scan webhook: %w", err)
		}
		h.Enabled = enabled != 0
		h.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", created)
		out = append(out, h)
	}
	return out, rows.Err()
}

func (s *Store) AddDiscordWebhook(h DiscordWebhook) (int64, error) {
	res, err := s.db.Exec(`
		INSERT INTO discord_webhooks (url, event_type, enabled) VALUES (?, ?, ?)
	`, h.URL, h.EventType, boolInt(h.Enabled))
	if err != nil {
		return 0, fmt.Errorf("add webhook: %w", err)
	}
	return res.LastInsertId()
}

func (s *Store) DeleteDiscordWebhook(id int) error {
	_, err := s.db.Exec("DELETE FROM discord_webhooks WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete webhook: %w", err)
	}
	return nil
}

func (s *Store) GetDiscordWebhooksByEvent(eventType string) ([]DiscordWebhook, error) {
	rows, err := s.db.Query(`
		SELECT id, url, event_type, enabled, created_at
		FROM discord_webhooks WHERE event_type = ? AND enabled = 1
	`, eventType)
	if err != nil {
		return nil, fmt.Errorf("get webhooks by event: %w", err)
	}
	defer rows.Close()

	var out []DiscordWebhook
	for rows.Next() {
		var h DiscordWebhook
		var enabled int
		var created string
		if err := rows.Scan(&h.ID, &h.URL, &h.EventType, &enabled, &created); err != nil {
			return nil, fmt.Errorf("scan webhook: %w", err)
		}
		h.Enabled = enabled != 0
		h.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", created)
		out = append(out, h)
	}
	return out, rows.Err()
}

func (s *Store) LogEvent(eventType, source string, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal event payload: %w", err)
	}
	_, err = s.db.Exec(`
		INSERT INTO event_log (event_type, source, payload) VALUES (?, ?, ?)
	`, eventType, source, string(data))
	if err != nil {
		return fmt.Errorf("log event: %w", err)
	}
	return nil
}

func (s *Store) ListEvents(limit int) ([]EventLog, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(`
		SELECT id, event_type, source, payload, created_at
		FROM event_log ORDER BY id DESC LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	defer rows.Close()

	var out []EventLog
	for rows.Next() {
		var e EventLog
		var created string
		if err := rows.Scan(&e.ID, &e.EventType, &e.Source, &e.Payload, &created); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		e.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", created)
		out = append(out, e)
	}
	return out, rows.Err()
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
