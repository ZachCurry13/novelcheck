# Hardcover API: what exists and what NovelCheck uses

Endpoint `https://api.hardcover.app/v1/graphql`, `Authorization: Bearer <token>`. Limits (Hardcover's own docs):
60 requests a minute, 30 s timeout, query depth 3; a user-agent that says what the script is is requested.
`status_id`: 1 Want to read, 2 Reading, 3 Read, 4 Paused, 5 DNF, 6 Ignored. `user_books.rating` is 0-5 and
**supports halves**. Checked 2026-11-09 against Hardcover's published `field-descriptions.json` and
`capabilities.json` (github.com/hardcoverapp/hardcover-docs) and the current official KOReader plugin and
Readest, which still make the same write calls we do.

## Used today
| Call | Where | For |
|---|---|---|
| `me { id username }` | `Username` | testing a token |
| `me { user_books(status in 2,3) { status_id rating book edition user_book_reads } }` | `Shelves` | pull: what you read and rated |
| `editions(where isbn_10/13)` | `FindEdition` | matching a book by ISBN |
| `me { user_books(where book_id) }` | `FindUserBook` | your entry for a book |
| `insert_user_book`, `update_user_book` (status, rating) | `Push`, `SetRating` | push status and stars |
| `insert_user_book_read`, `update_user_book_read` | `Push` | push progress and finish date |
| `series(where name)` with `book_series` | `SeriesBooks` | Find the missing books |

Since 1.59.1: calls are spaced 1.1 s apart (a book sent takes up to five calls; the limit is 60 a minute), a 429 waits
15 s and tries once more, calls send a `User-Agent`, and writes ask for `error` so a refused write is reported.

## Available and not used (ideas, most useful first)
1. **Want to read (status 1) both ways.** Pull it into the wishlist; push Nightstand books that are only queued.
2. **Paused (4) and DNF (5).** The Nightstand's "Back to Nightstand" could set Paused on Hardcover.
3. **`search(query, query_type: "Book")`.** Find a book without an ISBN (title and author, exact match only).
4. **`me { goals }`** (reading challenge): show yearly goal progress in Wrapped and trophies.
5. **`user_book_reads.started_at/finished_at`** and `date_added`: real finish dates for Wrapped.
6. **`insert_reading_journal`** (notes, quotes, progress events) for KOReader highlights.
7. **`reviews`** (`review_raw`, `has_review`), **lists** (`me { lists }`, `insert_list_book`), **ASIN** edition match
   (Kindle books), **`upsert_user_book_reads`** (newer name for the read calls; the old ones still work).
