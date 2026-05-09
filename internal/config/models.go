package config

import "time"

type Challenge struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	BaseFlag    string    `json:"base_flag"`
	LeetRules   string    `json:"leet_rules"`
	Questions   string    `json:"questions"`
	NetcatPort  int       `json:"netcat_port"`
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CTFdConfig struct {
	ID               int       `json:"id"`
	URL              string    `json:"url"`
	APIKey           string    `json:"api_key"`
	WebhookSecret    string    `json:"webhook_secret"`
	DetectedEdition  string    `json:"detected_edition"`
	PluginInstalled  bool      `json:"plugin_installed"`
	PollIntervalSec  int       `json:"poll_interval_sec"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type DiscordWebhook struct {
	ID        int       `json:"id"`
	URL       string    `json:"url"`
	EventType string    `json:"event_type"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
}

type EventLog struct {
	ID        int       `json:"id"`
	EventType string    `json:"event_type"`
	Source    string    `json:"source"`
	Payload   string    `json:"payload"`
	CreatedAt time.Time `json:"created_at"`
}
