package genres

import (
	"reflect"
	"testing"
)

func TestFromTags(t *testing.T) {
	cases := []struct {
		tags []string
		keys []string
		kind string
	}{
		{[]string{"Fiction", "Fantasy", "Romance"}, []string{"fantasy", "romance"}, "fiction"},
		{[]string{"Nonfiction", "History", "Biography"}, []string{"biography", "history"}, "nonfiction"},
		{[]string{"Science Fiction", "Alternate History"}, []string{"scifi"}, "fiction"},
		{[]string{"True Crime"}, []string{"truecrime"}, "nonfiction"},
		{[]string{"Young Adult", "Dystopian"}, []string{"scifi", "ya"}, "fiction"},
		{[]string{"Christian Fiction"}, []string{"religion"}, "fiction"},
		{[]string{"Poetry", "Universe"}, []string{"poetry"}, ""},
		{[]string{"Fiction / Mystery & Detective / Cozy"}, []string{"mystery"}, "fiction"},
		{[]string{"Cooking", "Baking"}, []string{"cooking"}, "nonfiction"},
		{[]string{"Dragons", "Magic"}, []string{"fantasy"}, "fiction"},
		{[]string{"Epub", "Calibre", "To read"}, nil, ""},
		{nil, nil, ""},
	}
	for _, c := range cases {
		keys, kind := FromTags(c.tags)
		if !reflect.DeepEqual(keys, c.keys) || kind != c.kind {
			t.Errorf("FromTags(%q) = %v %q, want %v %q", c.tags, keys, kind, c.keys, c.kind)
		}
	}
}

func TestJoinSplit(t *testing.T) {
	if Join(nil) != "" || Join([]string{"a", "b"}) != ",a,b," || !reflect.DeepEqual(Split(",a,b,"), []string{"a", "b"}) || Split("") != nil {
		t.Fatal("join/split")
	}
	if !Valid("fantasy") || Valid("nope") || Label("ya") != "Young Adult" {
		t.Fatal("lookups")
	}
}
