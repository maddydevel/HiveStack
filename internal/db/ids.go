package db

import "github.com/google/uuid"

// newID generates a new random identifier used for all HiveStack row IDs.
// IDs are stored as TEXT (rather than a native Postgres UUID column) so the
// schema has no dependency on the pgcrypto/uuid-ossp extensions.
func newID() string {
	return uuid.New().String()
}
