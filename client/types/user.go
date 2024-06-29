package types

import "time"

type Credential struct {
	Username       string    `toml:"username" json:"username"`
	Password       string    `toml:"password" json:"password"`
	Token          string    `toml:"token" json:"token"`
	TokenCreatedAt time.Time `toml:"token_created_at" json:"tokenCreatedAt"`
}
