package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// CategoryIDs is the set of categories a room's song pool is restricted to. An
// empty (or nil) value means every category, matching how the creation forms
// treat "All Categories" — so callers must skip the filter entirely rather than
// building a WHERE clause that matches nothing.
//
// Stored as jsonb, the same way the other composite room fields (TreeState,
// QuizState) persist.
type CategoryIDs []uint

// Scan implements sql.Scanner for CategoryIDs.
func (c *CategoryIDs) Scan(value interface{}) error {
	if value == nil {
		*c = nil
		return nil
	}

	var bytes []byte
	switch typed := value.(type) {
	case []byte:
		bytes = typed
	case string:
		bytes = []byte(typed)
	default:
		return fmt.Errorf("cannot scan %T into CategoryIDs", value)
	}

	if len(bytes) == 0 {
		*c = nil
		return nil
	}
	return json.Unmarshal(bytes, c)
}

// Value implements driver.Valuer for CategoryIDs.
func (c CategoryIDs) Value() (driver.Value, error) {
	if len(c) == 0 {
		return nil, nil
	}
	return json.Marshal(c)
}

// Resolve returns the category IDs a room's song pool is limited to, empty
// meaning unrestricted.
//
// legacy is the room's single-category CategoryID column, which is all that
// rooms created before multi-select was added carry. Those rooms must keep
// filtering the way they were created, so it is used whenever no multi-select
// value was stored.
func (c CategoryIDs) Resolve(legacy *uint) []uint {
	if len(c) > 0 {
		return c
	}
	if legacy != nil {
		return []uint{*legacy}
	}
	return nil
}
