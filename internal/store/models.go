// Package store holds the data-access layer for the NovelCheck database.
// Queries always materialise results (Select/Get) because the pool is capped
// at one connection; never nest queries inside a rows iteration.
package store

import "github.com/jmoiron/sqlx"

type Store struct {
	DB *sqlx.DB
	// OnNotify, if set, hears about each new notification (not repeats of an
	// unread one), e.g. to push it to phones. It must not block.
	OnNotify func(level, source, message, link string)
}

func New(db *sqlx.DB) *Store { return &Store{DB: db} }

// Roles, from most to least privileged:
//   - admin:      everything, including API keys, SMTP, library folder, backups
//   - editor:     day-to-day management (scans, verdict corrections, drive
//     imports, kids' accounts) without the technical settings
//   - restricted: kid account, filtered by its content rules
const (
	RoleAdmin      = "admin"
	RoleEditor     = "editor"
	RoleRestricted = "restricted"
)

// ValidRole reports whether r is a known role.
func ValidRole(r string) bool { return r == RoleAdmin || r == RoleEditor || r == RoleRestricted }

type User struct {
	ID             int64  `db:"id" json:"id"`
	Username       string `db:"username" json:"username"`
	PasswordHash   string `db:"password_hash" json:"-"`
	Role           string `db:"role" json:"role"`
	HideOpenDoor   bool   `db:"hide_open_door" json:"hide_open_door"`
	HideNudity     bool   `db:"hide_nudity" json:"hide_nudity"`
	HideSoloActs   bool   `db:"hide_solo_acts" json:"hide_solo_acts"`
	HideInnuendo   bool   `db:"hide_innuendo" json:"hide_innuendo"`
	HideDarkOccult bool   `db:"hide_dark_occult" json:"hide_dark_occult"`
	HideLGBTQ      bool   `db:"hide_lgbtq" json:"hide_lgbtq"`
	HideUnrated    bool   `db:"hide_unrated" json:"hide_unrated"`
	DeliveryMethod string `db:"delivery_method" json:"delivery_method"`
	KindleEmail    string `db:"kindle_email" json:"kindle_email"`
	GuideSeen      bool   `db:"guide_seen" json:"guide_seen"`
	AgeLevel       int    `db:"age_level" json:"age_level"` // kid accounts only; 0 = not set
	MaxSpice       int    `db:"max_spice" json:"max_spice"` // kid accounts: most peppers shown (0-5); -1 = no limit
	CreatedAt      string `db:"created_at" json:"created_at"`
}

func (u *User) IsAdmin() bool { return u.Role == RoleAdmin }

// CanManage is true for admins and editors (the non-technical management tier).
func (u *User) CanManage() bool { return u.Role == RoleAdmin || u.Role == RoleEditor }

type Catalog struct {
	ID        int64  `db:"id" json:"id"`
	Name      string `db:"name" json:"name"`
	Source    string `db:"source" json:"source"`
	CreatedAt string `db:"created_at" json:"created_at"`
	BookCount int    `db:"book_count" json:"book_count"`
}

type Book struct {
	ID              int64   `db:"id" json:"id"`
	NormKey         string  `db:"norm_key" json:"-"`
	Title           string  `db:"title" json:"title"`
	Author          string  `db:"author" json:"author"`
	ISBN            string  `db:"isbn" json:"isbn"`
	Description     string  `db:"description" json:"description"`
	Blurb           string  `db:"blurb" json:"blurb"`
	Status          string  `db:"status" json:"status"`
	Classification  *string `db:"classification" json:"classification"`
	Nudity          bool    `db:"nudity" json:"nudity"`
	SoloActs        bool    `db:"solo_acts" json:"solo_acts"`
	HeavyInnuendo   bool    `db:"heavy_innuendo" json:"heavy_innuendo"`
	PlayfulFantasy  bool    `db:"playful_fantasy" json:"playful_fantasy"`
	DarkOccult      bool    `db:"dark_occult" json:"dark_occult"`
	DemonicPresence bool    `db:"demonic_presence" json:"demonic_presence"`
	LGBTQContent    bool    `db:"lgbtq_content" json:"lgbtq_content"`
	SummaryVerdict  string  `db:"summary_verdict" json:"summary_verdict"`
	Approved        bool    `db:"approved" json:"approved"`
	ApprovedBy      string  `db:"approved_by" json:"approved_by"`
	AgeLevel        int     `db:"age_level" json:"age_level"`
	SpiceLevel      *int    `db:"spice_level" json:"spice_level"`   // 0-5 peppers; nil = not on the pepper scale yet
	SpiceReason     string  `db:"spice_reason" json:"spice_reason"` // what drives the pepper level, a few words
	AgeSetBy        string  `db:"age_set_by" json:"age_set_by"`
	AnalysisModel   string  `db:"analysis_model" json:"analysis_model"`
	AnalysisError   string  `db:"analysis_error" json:"analysis_error"`
	Series          string  `db:"series" json:"series"`             // series name, when known
	SeriesIndex     float64 `db:"series_index" json:"series_index"` // number in the series; 0 = none
	TitleFix        string  `db:"title_fix" json:"title_fix"`       // Calibre's untidy title ("01 - Dune"); "" = tidy
	Tags            string  `db:"tags" json:"tags"`                 // Calibre's tags, comma-separated
	Genres          string  `db:"genres" json:"genres"`             // categories as ",fantasy,romance," (package genres)
	Kind            string  `db:"kind" json:"kind"`                 // fiction, nonfiction or ""
	GenreSource     string  `db:"genre_source" json:"genre_source"` // calibre, ai or ""
	AnalyzedAt      *string `db:"analyzed_at" json:"analyzed_at"`
	CreatedAt       string  `db:"created_at" json:"created_at"`
	UpdatedAt       string  `db:"updated_at" json:"updated_at"`
	Catalogs        string  `db:"catalogs" json:"catalogs"`               // comma-joined names (list queries)
	Formats         string  `db:"formats" json:"formats"`                 // e.g. "AZW3,EPUB": file formats across copies
	CalibreCopies   int     `db:"calibre_copies" json:"calibre_copies"`   // Calibre entries for this book (2+ = duplicate)
	DeleteRequests  int     `db:"delete_requests" json:"delete_requests"` // pending requests to delete it
	CustomFlags     string  `db:"custom_flags" json:"custom_flags"`       // keys of the family's filters it matches, comma-separated
}

// BookCopy is one physical copy of a book inside a catalog.
type BookCopy struct {
	CatalogID   int64  `db:"catalog_id" json:"catalog_id"`
	CatalogName string `db:"catalog_name" json:"catalog_name"`
	Source      string `db:"source" json:"source"`
	Path        string `db:"path" json:"path"`
	Format      string `db:"format" json:"format"`
	ExternalID  string `db:"external_id" json:"external_id"`
}

// Analysis is the structured LLM verdict persisted onto a book.
type Analysis struct {
	SpiceLevel      *int // 0-5 peppers; nil for old-style verdicts
	SpiceReason     string
	Classification  string
	Nudity          bool
	SoloActs        bool
	HeavyInnuendo   bool
	PlayfulFantasy  bool
	DarkOccult      bool
	DemonicPresence bool
	LGBTQContent    bool
	SummaryVerdict  string
	Model           string
	CustomFlags     []string // keys of the family's filters the book matches
}

type QueueItem struct {
	ID             int64   `db:"id" json:"id"`
	BookID         int64   `db:"book_id" json:"book_id"`
	Position       int     `db:"position" json:"position"`
	Status         string  `db:"status" json:"status"`
	DeliveryNote   string  `db:"delivery_note" json:"delivery_note"`
	UpdatedAt      string  `db:"updated_at" json:"updated_at"`
	Title          string  `db:"title" json:"title"`
	Author         string  `db:"author" json:"author"`
	Classification *string `db:"classification" json:"classification"`
	BookStatus     string  `db:"book_status" json:"book_status"`
	SpiceLevel     *int    `db:"spice_level" json:"spice_level"`
	DeepChange     string  `db:"deep_change" json:"deep_change"` // e.g. "2→4" when a Deep Scan raised the rating
	Owned          bool    `db:"owned" json:"owned"`             // false = only looked up: still to get
}
