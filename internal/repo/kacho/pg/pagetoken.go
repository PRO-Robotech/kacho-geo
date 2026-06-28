// Copyright (c) PRO-Robotech
// SPDX-License-Identifier: BUSL-1.1

package pg

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// pageCursor — непрозрачный курсор, кодируемый в page_token (base64 JSON).
// Region/Zone List сортирует и фильтрует по id (WHERE id > $1), поэтому курсор —
// id-only: created_at в токене не использовался (мертвый payload) и убран.
type pageCursor struct {
	ID string `json:"id"`
}

// encodePageToken собирает непрозрачный base64-курсор из id.
func encodePageToken(id string) string {
	b, _ := json.Marshal(pageCursor{ID: id})
	return base64.StdEncoding.EncodeToString(b)
}

// decodePageToken разбирает непрозрачный курсор; возвращает (id, err).
func decodePageToken(token string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return "", err
	}
	var c pageCursor
	if err := json.Unmarshal(raw, &c); err != nil {
		return "", err
	}
	if c.ID == "" {
		return "", fmt.Errorf("empty cursor id")
	}
	return c.ID, nil
}

// invalidPageTokenErr превращает мусорный page_token в gRPC InvalidArgument
// (garbage page_token → InvalidArgument).
func invalidPageTokenErr(err error) error {
	return status.Errorf(codes.InvalidArgument, "invalid page_token: %v", err)
}
