package graph

import (
	"fmt"
	"sort"
)

// Migration transforms a Document loaded from version v to v+1.
type Migration func(doc *Document) error

var migrations = map[int]Migration{}

// RegisterMigration registers an upgrade step from fromVersion -> fromVersion + 1.
// Keeping migrations additive ensures forward/backward compatibility across releases.
func RegisterMigration(fromVersion int, fn Migration) {
	if fromVersion < 1 || fn == nil {
		return
	}
	migrations[fromVersion] = fn
}

// MigrateUp upgrades doc sequentially through all registered schema migrations up to FormatVersion.
// Documents with FormatVersion > current binary's FormatVersion are preserved untouched (with
// unknown Meta/Kind values kept intact) so newer files remain inspectable by older builds.
func MigrateUp(doc *Document) error {
	if doc == nil {
		return nil
	}
	if doc.FormatVersion <= 0 {
		doc.FormatVersion = 1
	}
	if doc.FormatVersion > FormatVersion {
		// Newer schema created by a future DrawGraph version: preserve unknown metadata cleanly.
		return nil
	}
	keys := make([]int, 0, len(migrations))
	for k := range migrations {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	for doc.FormatVersion < FormatVersion {
		fn, ok := migrations[doc.FormatVersion]
		if !ok {
			doc.FormatVersion++
			continue
		}
		if err := fn(doc); err != nil {
			return fmt.Errorf("migrate document v%d->v%d: %w", doc.FormatVersion, doc.FormatVersion+1, err)
		}
		doc.FormatVersion++
	}
	return nil
}
