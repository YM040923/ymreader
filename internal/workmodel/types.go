package workmodel

// UnitKind classifies how a physical readable item fits inside a work.
type UnitKind string

const (
	UnitKindFull    UnitKind = "full"
	UnitKindChapter UnitKind = "chapter"
	UnitKindVolume  UnitKind = "volume"
	UnitKindExtra   UnitKind = "extra"
	UnitKindSpecial UnitKind = "special"
	UnitKindUnknown UnitKind = "unknown"
)

// SortKey is a stable, pure ordering key for units in a work.
type SortKey struct {
	KindRank int
	Number   float64
	Raw      string
}

// ResolvedUnit is the parsed unit metadata for a single source path.
type ResolvedUnit struct {
	Kind          UnitKind
	Title         string
	DisplayLabel  string
	VolumeNumber  *float64
	ChapterNumber *float64
	SortKey       SortKey
}

// ResolvedPath is the parsed work/unit view of one physical path.
type ResolvedPath struct {
	RelativePath string
	WorkTitle    string
	Unit         ResolvedUnit
}

func floatPtr(v float64) *float64 { return &v }
