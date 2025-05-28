package core

// GlobalAppConfig holds application-wide configurations.
// These are typically set at startup in main.go.
var GlobalAppConfig struct {
	RootDir   string
	FlagHost  string
	MetaHost  string // Added as it's used in apiList's JavResp logic
	TrashPath string // Path to the .Trash directory
	RedisHost string // Redis host address (e.g., "localhost:6379")
	// Add other global configs as needed
}
