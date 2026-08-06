package store

func init() {
	Migrations = append(Migrations, Migration{
		Version:     43,
		Description: "Persist Work-level publication status on standalone comics",
		SQL:         `ALTER TABLE "Comic" ADD COLUMN "status" TEXT NOT NULL DEFAULT '';`,
	})
}
