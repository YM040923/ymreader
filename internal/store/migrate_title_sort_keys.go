package store

import "strings"

func init() {
	Migrations = append(Migrations, Migration{
		Version:     48,
		Description: "Rebuild natural Chinese title sort keys for comics and logical works",
		SQL: strings.Join([]string{
			`UPDATE "Comic" SET "titleSortKey" = title_sort_key("title") WHERE "title" <> '';`,
			`UPDATE "LogicalWork" SET "sortTitle" = title_sort_key("title") WHERE "title" <> '';`,
		}, "\n"),
	})
}
