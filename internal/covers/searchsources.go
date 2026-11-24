package covers

import (
	"context"
	"net/url"
	"strings"
)

// openLibraryCoverURL is the cover of the first Open Library book with this title and author, and an ISBN that
// book carries ("" when it has none).
func (c Cache) openLibraryCoverURL(ctx context.Context, title, author string, refused *bool) (string, string) {
	base := c.SearchURL
	if base == "" {
		base = "https://openlibrary.org"
	}
	q := url.Values{"title": {title}, "limit": {"15"}, "fields": {"title,author_name,cover_i,isbn"}}
	if lastName(author) != "" {
		q.Set("author", mainAuthor(author)) // "Rob Knight with Brendan Buhler" is searched as Rob Knight
	}
	var res struct {
		Docs []struct {
			Title  string   `json:"title"`
			Author []string `json:"author_name"`
			Cover  int64    `json:"cover_i"`
			ISBN   []string `json:"isbn"`
		} `json:"docs"`
	}
	if !c.getJSON(ctx, base+"/search.json?"+q.Encode(), &res, refused) {
		return "", ""
	}
	cover := c.OpenLibraryURL
	if cover == "" {
		cover = openLibrary
	}
	isbn := ""
	for _, d := range res.Docs {
		if !sameTitle(d.Title, title) || !authorOK(d.Author, author) {
			continue
		}
		if isbn == "" {
			isbn = bestISBN(d.ISBN)
		}
		if d.Cover > 0 {
			return cover + "/b/id/" + url.PathEscape(itoa(d.Cover)) + "-L.jpg?default=false", isbn
		}
	}
	return "", isbn
}

// googleCoverURL is the cover of the first Google Books volume with this title and author (at about 600 pixels),
// and an ISBN that volume carries.
func (c Cache) googleCoverURL(ctx context.Context, title, author string, refused *bool) (string, string) {
	base := c.GoogleURL
	if base == "" {
		base = "https://www.googleapis.com"
	}
	q := "intitle:" + title
	if lastName(author) != "" {
		q += " inauthor:" + mainAuthor(author)
	}
	var res struct {
		Items []struct {
			Info struct {
				Title   string   `json:"title"`
				Authors []string `json:"authors"`
				IDs     []struct {
					Type string `json:"type"`
					ID   string `json:"identifier"`
				} `json:"industryIdentifiers"`
				Images struct {
					Thumb string `json:"thumbnail"`
				} `json:"imageLinks"`
			} `json:"volumeInfo"`
		} `json:"items"`
	}
	params := url.Values{"q": {q}, "maxResults": {"6"}}
	if c.GoogleKey != nil {
		if key := strings.TrimSpace(c.GoogleKey()); key != "" {
			params.Set("key", key)
		}
	}
	if !c.getJSON(ctx, base+"/books/v1/volumes?"+params.Encode(), &res, refused) {
		return "", ""
	}
	isbn := ""
	for _, it := range res.Items {
		if !sameTitle(it.Info.Title, title) || !authorOK(it.Info.Authors, author) {
			continue
		}
		if isbn == "" {
			var ids []string
			for _, id := range it.Info.IDs {
				if strings.HasPrefix(id.Type, "ISBN") {
					ids = append(ids, id.ID)
				}
			}
			isbn = bestISBN(ids)
		}
		if it.Info.Images.Thumb != "" {
			u := strings.Replace(it.Info.Images.Thumb, "http://", "https://", 1)
			u = strings.Replace(u, "zoom=1", "zoom=3", 1) // Google's thumbnail is 128 pixels; zoom=3 is about 600
			return strings.ReplaceAll(u, "&edge=curl", ""), isbn
		}
	}
	return "", isbn
}

// bestISBN picks a 13-digit ISBN from a book's list when it has one, else a 10-digit one ("" for neither).
func bestISBN(ids []string) string {
	ten := ""
	for _, id := range ids {
		id = strings.ToUpper(strings.NewReplacer("-", "", " ", "").Replace(id))
		if !isbnRE.MatchString(id) {
			continue
		}
		if len(id) == 13 {
			return id
		}
		if ten == "" {
			ten = id
		}
	}
	return ten
}
