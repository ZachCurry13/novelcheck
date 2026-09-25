// Genre names (the same list as internal/genres) and links that open the
// library filtered to a genre, author or series.
import { esc } from "./ui.js";

export const GENRES = {
  fantasy: "Fantasy", scifi: "Science Fiction", romance: "Romance", mystery: "Mystery & Thriller", horror: "Horror",
  historical: "Historical Fiction", literary: "Literary & Contemporary", classics: "Classics", adventure: "Adventure",
  humor: "Humor", ya: "Young Adult", children: "Children's", graphic: "Comics & Graphic Novels", poetry: "Poetry",
  religion: "Religion & Spirituality", biography: "Biography & Memoir", history: "History", truecrime: "True Crime",
  selfhelp: "Self-help & Wellbeing", science: "Science & Nature", business: "Business & Money", cooking: "Cooking & Food",
};

const link = (param, value) => `#/library?${param}=${encodeURIComponent(value)}`;

// genreChips links a book's fiction/nonfiction and genres to the library.
export function genreChips(b) {
  const keys = (b.genres || "").split(",").filter((k) => GENRES[k]);
  const kind = b.kind ? `<a href="${link("kind", b.kind)}" data-close class="chip-cat">${b.kind === "fiction" ? "Fiction" : "Nonfiction"}</a>` : "";
  const byAI = b.genre_source === "ai" ? ` title="Genre suggested by the AI (Calibre has no tags for this book)"` : "";
  return kind + keys.map((k) => `<a href="${link("genre", k)}" data-close class="chip-cat"${byAI}>${esc(GENRES[k])}</a>`).join("");
}

// authorLinks makes each author of "A & B" a link to their books.
export const authorLinks = (author) => (author || "Unknown author").split(" & ").map((a) => author
  ? `<a href="${link("author", a.trim())}" data-close class="hover:underline">${esc(a.trim())}</a>` : esc(a)).join(" &amp; ");

export const seriesLink = (b, text) => `<a href="${link("series", b.series)}" data-close class="text-sm text-sky-300 hover:underline">📚 ${esc(text)}</a>`;
