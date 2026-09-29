package db

import (
	"database/sql/driver"

	"github.com/zachcurry13/novelcheck/internal/titles"
	"modernc.org/sqlite"
)

// SQL functions for sorting: sort_title(title) drops a leading "The/A/An";
// sort_author(author) puts the first author's last name first.
func init() {
	text := func(fn func(string) string) func(*sqlite.FunctionContext, []driver.Value) (driver.Value, error) {
		return func(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			s, _ := args[0].(string)
			return fn(s), nil
		}
	}
	sqlite.MustRegisterDeterministicScalarFunction("sort_title", 1, text(titles.SortTitle))
	sqlite.MustRegisterDeterministicScalarFunction("sort_author", 1, text(titles.SortAuthor))
}
