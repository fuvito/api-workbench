package serialization

import (
	"fmt"

	"github.com/fuvito/api-workbench/packages/core/domain"
)

// checkSchemaVersion refuses a file written for a format this version does not know: reading it as
// the current one would silently ignore whatever is new in it. A file with no schemaVersion is read
// as before, because such files have always worked.
func checkSchemaVersion(v string) error {
	if v == "" || v == domain.CurrentSchemaVersion {
		return nil
	}
	return fmt.Errorf("schemaVersion %q is not supported (this version of clrsig reads %q; a newer file needs a newer clrsig)", v, domain.CurrentSchemaVersion)
}
