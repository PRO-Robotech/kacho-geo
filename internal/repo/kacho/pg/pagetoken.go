package pg

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// pageCursor — opaque cursor payload encoded into page_token (base64 JSON).
// Region/Zone List orders by id ASC; created_at is carried for forward-compat
// with the (created_at, id) convention but the WHERE clause keys on id.
type pageCursor struct {
	CreatedAt time.Time `json:"created_at"`
	ID        string    `json:"id"`
}

// encodePageToken builds an opaque base64 cursor from (created_at, id).
func encodePageToken(createdAt time.Time, id string) string {
	b, _ := json.Marshal(pageCursor{CreatedAt: createdAt, ID: id})
	return base64.StdEncoding.EncodeToString(b)
}

// decodePageToken parses an opaque cursor; returns (created_at, id, err).
func decodePageToken(token string) (time.Time, string, error) {
	raw, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return time.Time{}, "", err
	}
	var c pageCursor
	if err := json.Unmarshal(raw, &c); err != nil {
		return time.Time{}, "", err
	}
	if c.ID == "" {
		return time.Time{}, "", fmt.Errorf("empty cursor id")
	}
	return c.CreatedAt, c.ID, nil
}

// invalidPageTokenErr maps a garbage page_token into a gRPC InvalidArgument
// (api-conventions.md: garbage token → InvalidArgument).
func invalidPageTokenErr(err error) error {
	return status.Errorf(codes.InvalidArgument, "invalid page_token: %v", err)
}
