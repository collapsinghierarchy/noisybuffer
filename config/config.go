package config

type Config struct {
	MaxBlobBytes    int64 // e.g. 64*1024
	AllowedKEMs     []string
	RateLimitBurst  int
	RateLimitMinute int
}
