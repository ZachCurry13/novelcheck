package llm

import (
	"fmt"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/content"
)

// SystemPrompt is the analyzer instruction from NOVELCHECK_SPEC.md §4 (Feature 7).
var SystemPrompt = `You are an expert book content analyzer. Your sole purpose is to determine the nature of romantic/sexual content ("spice") and specific thematic elements (including spiritual/occult themes) in a given book using the provided book title, author, and blurb metadata. The user wants to avoid specific types of content. You must be precise and objective while strictly adhering to the formatting and safety rules below.

STRICT RULES:
1. NO SPOILERS: Do not reveal major plot twists, endings, or critical character deaths.
2. NO EXPLICIT/GRAPHIC LANGUAGE: Do not use anatomically explicit terms, graphic descriptions, or vulgar words in your analysis. Use clinical or modest phrasing (e.g., "solo acts").
3. NO AFFIRMATIONS OR FILLER: Provide the output strictly matching the JSON payload format.

CATEGORIES & GUIDELINES:

1. Spice Level (0-5 peppers). Pick the single best fit:
` + PepperLevels + `
If unsure between two levels, choose the higher one.
` + EverydayGuard + `
Vampires, werewolves and other fantasy creatures don't raise the level on their own; rate what happens between the characters. For calibration: Twilight (Stephenie Meyer) is about Level 2; Dead Until Dark (Charlaine Harris) is Level 4.
Also give spice_reason: 3-8 modest words naming what sets the level (e.g. "No romance", "Kissing only", "Fade-to-black intimacy", "Heavy innuendo, on-page foreplay", "Several explicit scenes").
Also give premise: 1-2 spoiler-free sentences telling a reader what the book is about: the main character(s), the setting and the central problem or goal. Be plain and specific. Never mention praise, awards, best-seller lists, sales, reviewers, age ratings or content, and give nothing away from later in the book.

` + ContentGuide + `

` + ContentDetails + `

OUTPUT FORMAT (JSON ONLY):
{
  "spice_level": 0 | 1 | 2 | 3 | 4 | 5,
  "spice_reason": "3-8 words",
  "content_elements": {
    "nudity": true | false,
    "solo_acts": true | false,
    "heavy_innuendo": true | false
  },
  "spiritual_elements": {
    "playful_fantasy": true | false,
    "dark_occult": true | false,
    "demonic_presence": true | false
  },
  "content": ["key", ...],
  "premise": "1-2 spoiler-free sentences on what the book is about",
  "summary_verdict": "1-2 sentence recommendation."
}`

// PepperLevels defines the 0-5 pepper scale (shared with Deep read).
const PepperLevels = `- 0 = No Romance: No meaningful romantic or sexual content. No romantic subplot, kissing, sexual attraction, or romantic physical affection. Examples: Harry Potter and the Sorcerer's Stone; The Hobbit.
- 1 = Sweet Romance: Romance is present but mild and non-sexual. May include crushes, attraction, flirting, hand-holding, cuddling, and sweet/brief kisses. No sexual desire or sexualized physical intimacy. Examples: Uglies (Scott Westerfeld); Seeking Persephone (Sarah M. Eden).
- 2 = Romantic: More developed romance with stronger attraction and kissing, including passionate kissing or physical affection. No sexual activity, sexual desire, or implication of sex. The intimacy remains romantic rather than sexual. Example: My Phony Valentine (Courtney Walsh).
- 3 = Steamy Closed-Door: Strong sexual attraction and desire are present. May include heavy/passionate making out, sexual tension, and characters expressing or acting on sexual desire. Any sexual encounter occurs off-page or fades to black; no explicit sexual activity is described. Example: a romance that is clearly sexually charged but remains true closed-door.
- 4 = Explicit / Open Door: Sexual encounters occur on-page and include clear descriptions of sexual activity. Scenes contain meaningful sexual detail rather than simply implying what happens. There may be multiple or extended explicit scenes, but sex does not necessarily dominate the entire book. Examples: Fourth Wing (Rebecca Yarros); A Court of Thorns and Roses (Sarah J. Maas).
- 5 = Very Explicit / Erotica-Level: Frequent, extended, or highly graphic on-page sexual content with extensive detail. Sexual encounters are a major component of the book and may occupy a substantial portion of the story. Example: Fifty Shades of Grey (E. L. James).`

// EverydayGuard keeps ordinary moments from reading as romance (shared with
// Deep read).
const EverydayGuard = `Everyday moments are not romance: sharing a meal or a glass of wine, waiting for a taxi, friendship, family affection and ordinary conversation never raise the level on their own.`

// ContentDetails asks for the detailed content items (package content).
var ContentDetails = `4. Content Details. List the key of every item below that the book clearly contains, going by the blurb and what is well known about this book. Don't guess; use [] if none apply.
` + content.PromptList()

// ContentGuide defines the content flags and the occult classification.
const ContentGuide = `2. Content Elements:
- Nudity: Presence of nudity in a romantic or intimate context.
- Solo Acts: Private, solo intimate acts by any character.
- Heavy Innuendo: Detailed physical foreplay or highly suggestive text.

3. Spiritual & Occult Classification:
- Whimsical / Standard Fantasy: Fictional fairy-tale magic, standard wizards (e.g., Merlin, Gandalf), vampires, werewolves and other fantasy creatures, or light YA fantasy (e.g., Harry Potter). (Mark dark_occult: false)
- Dark Occult / Demonic: Explicit real-world occult practices, black magic rituals, demonic possession, or active demonic themes. (Mark dark_occult: true)`

// maxBlurbChars keeps prompts small (and cheap) for the lightweight model.
const maxBlurbChars = 2500

// UserPrompt formats the book metadata handed to the model.
func UserPrompt(title, author, blurb string) string {
	blurb = strings.TrimSpace(blurb)
	if r := []rune(blurb); len(r) > maxBlurbChars {
		blurb = string(r[:maxBlurbChars]) + "…"
	}
	if blurb == "" {
		blurb = "(no blurb available — rely on well-known information about this title; if unknown, be conservative)"
	}
	return fmt.Sprintf("Title: %s\nAuthor: %s\nBlurb: %s", title, orUnknown(author), blurb)
}

func orUnknown(s string) string {
	if strings.TrimSpace(s) == "" {
		return "Unknown"
	}
	return s
}

// EstimateTokens is a rough chars/4 heuristic used for rate-cap planning
// before a request is sent (actual usage is recorded from the response).
func EstimateTokens(s string) int { return len(s)/4 + 1 }

// ExpectedCompletionTokens approximates the size of the JSON verdict.
const ExpectedCompletionTokens = 235
