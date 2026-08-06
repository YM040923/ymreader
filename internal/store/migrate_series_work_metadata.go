package store

import "strings"

func init() {
	Migrations = append(Migrations, Migration{
		Version:     40,
		Description: "Persist unified Work metadata source and cover aspect ratio on directory series",
		SQL: strings.Join([]string{
			`ALTER TABLE "ComicSeries" ADD COLUMN "metadataSource" TEXT NOT NULL DEFAULT '';`,
			`ALTER TABLE "ComicSeries" ADD COLUMN "coverAspectRatio" REAL NOT NULL DEFAULT 0;`,
		}, "\n"),
	})
}
