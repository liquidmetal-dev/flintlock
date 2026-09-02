package sqlite

// Config holds the sqlite repository configuration.
type Config struct {
	// DatabasePath is the path to the sqlite database file to use for storing
	// microvm spec definitions.
	DatabasePath string
}
