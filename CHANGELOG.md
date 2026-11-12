# Changelog

All notable changes to NovelCheck. Newest first. Each `## [x.y.z]` section
becomes the release notes for that version and is shown in the app under
**What's new**.

## [1.73.1]

### 🗑️ Calibre's recycle bin, checked against your library
- **Each removed book says whether you still have it.** "Missing from your library" means no copy of that book is in Calibre now: those are the ones that probably should not have gone. "Still in your library" means another copy is there (with its Calibre number), so an extra copy was removed. Empty entries have nothing to put back.
- **Simple to work with.** The bin opens with one line (how many books are missing), one Put back all button, and just the missing books: title, author, the day it went, and Put back. The books you still have are one tap away; the removal log and the rest are folded under Details.
- **Put back all missing.** One button puts every missing book back in Calibre, one entry per book (the EPUB when there is one), with a count as it goes and a list of any that did not work.

### 📊 Reading stats: one count everywhere
- **"Books finished" is the same number on every page.** Highlights, Records, Wrapped, trophies, the Profile summary and the Library banner now count from one list: each finished book once, on the day it was finished (the last day you read it in KOReader, or its finish day from an imported list, Hardcover or "I read this"). Before, a book read across two months counted as finished in both, and Wrapped added Hardcover's books its own way, so one page could say 11 and another 12.
- **The Hardcover goal says why it can differ.** The goal and its count are Hardcover's; when NovelCheck counts a different number, the slide says so (Hardcover only counts books marked read there).
- **Books KOReader finished without reading hours** now count on the day KOReader last opened them, so the all-time and yearly counts agree.
- **Wrapped works for a reader whose books come from a list**, and its longest and shortest books are the ones finished in that period.

### 🧬 Deep Scan: every part shows, and a second look
- **The review lists every part the scan noted**, not only Level 3 and up. A Level 1 or 2 suggestion used to show "What the AI found (0 parts)", which made it look like a mistake; now it shows the parts behind it. New scans also note Level 1 parts.
- **A second look at books kept too low.** The review tab lists the books whose scan found Level 1 or 2 but were kept at a lower level, and puts them back in the review with one button, so each can be decided again with its parts in view.

## [1.73.0]

### 📖 Read in NovelCheck
- **A reader inside the app.** A book's window has a **Read** button when the book has an EPUB, MOBI, AZW3 or FB2 in Calibre. The book opens full screen on your phone or computer.
- **Laid out like KOReader.** Tap the right two thirds of the page to go forward and the left third to go back (or swipe, or use the arrow keys); tap the top of the page for the menu. A slim line at the bottom shows the chapter, how far you are and the minutes left in the chapter.
- **Settings like KOReader's:** the book's own font, serif, sans or OpenDyslexic; text size; line spacing; margins; justified or ragged text; and day, sepia or night pages. They are kept on each device.
- **Contents:** the book's table of contents, one tap to a chapter.
- **Keeps your place.** Close the book and it opens where you left off, on any device. When KOReader's progress sync has you further on, it offers to jump there ("Your Kindle is at 54%"). The match is by percentage, so it lands within about a page.
- **…and sends your place back to your Kindle.** As you read, NovelCheck saves your place the way KOReader names it (the paragraph the page starts on) for the same EPUB file, through the progress sync your Kindle already uses. Open the book on the Kindle and KOReader's sync takes you there (or asks first, depending on KOReader's sync settings). It works when the Kindle has the same EPUB file as Calibre, for example one downloaded through NovelCheck.
- **Safe:** a book's own styles and fonts show, but any script inside a book is blocked.

### 🎨 Warmer: Paperback by day, Ember by night
- **A new look, chosen by the family.** The light theme is Paperback: a cream page, dark-blue ink and red-orange buttons. The dark theme is Ember: a warm near-black with a burnt-orange glow. Headings and book titles are in Literata, a bookish serif; everything else in Instrument Sans. Both fonts are stored with the app.
- **"Read this year" on the Library.** A banner at the top shows how many books you finished this year (KOReader, imported lists, Hardcover and "I read this" all count) and how that compares with last year. Tap it for Reading stats.
- **Themes have their own folder.** Every colour and font a theme decides is in `web/static/themes` (`paperback.css`, `ember.css`), with a guide to what each setting changes, so the look can be adjusted without touching the rest of the app.

## [1.72.1]

### 🖼️ Find covers
- **"Find covers now" looks at every book again.** A book searched in the last week with nothing found was skipped without a word, which is why a second run right after the first found 0 of 213. Pressing the button now searches them all again; the daily run still skips them, and its message says how many it skipped and why.

### 🗑️ Calibre's recycle bin
- **It no longer stops with an error** when the bin holds an entry with no book files in it.


## [1.72.0]

### 🎨 Calmer screens, everywhere
- **Grey, not navy.** The dark theme's surfaces are a neutral graphite, and the light theme a neutral white and grey, so book covers and the 0 to 5 level colours are what stand out.
- **One main button per screen.** The main action is an off-white button (dark on the light theme); the next one is a quiet grey button. The indigo accent now only marks the page you are on, links and the odd key figure.
- **No emoji as decoration.** Headings, buttons, tabs, status chips and messages lost about 480 decorative emoji. They stay where they are the content: trophies, awards, Wrapped, content flags and seasonal shelves.
- **Lighter headings and sentence case.** Page titles get their weight from size, labels are no longer in capitals, and messages lost their exclamation marks.
- **The Wrapped card** lost its purple glow.
- **Admin → Rating errors** shows each error in full, in plain text, with its buttons under it (it was squeezed into a narrow column, one word per line).
- **Collections and Series** switch with the same tabs as every other page.
- **Nightstand** labels are in sentence case and its notes lost their emoji.
- **Library cleanup and Reading sync** show a small coloured mark for done, to do and needs a look, in place of ✅ ⬜ ⚠️.
- **Suggested Reads** use proper thumbs-up and thumbs-down icons.
- **Users and content rules** are written in sentence case ("Hide open door").
- The design guide (`docs/DESIGN.md`) records the direction, from a review with the [unslop](https://github.com/heydamir/unslop) skill by Damir Sokolovsky.


## [1.71.2]

### ✓ Count a book you read yourself
- **"I read this" on every book.** In a book's window, press **✓ I read this** and pick the day you finished it. The book then counts in Reading stats for that year: a paper book, a library loan, or one an imported list missed. **Change** picks another day and **Not read** takes it out again.
- **Finished books with no day.** Goodreads leaves "Date Read" empty when you never set one, so those books couldn't be counted in any year. Reading stats now lists them under Records (and Highlights, when your reading comes from a list) with a box for the day. Give each one its day, a guess is fine, and it counts.


## [1.71.1]

### 🖼️ Find covers
- **The cover finder finds covers again.** It searched with the title exactly as Calibre has it, so "Percy Jackson 2 - The Sea of Monsters", "01 - Guards! Guards!" or "Stars Above (The Lunar Chronicles)" never matched. It now also tries the title without its series name or number, and without a subtitle.
- **Google Books is asked with your own API key** (Admin → Delivery & Services). Without it Google's shared daily limit was always used up, so every search there was refused.
- **Books it missed before are tried again.** A miss used to be remembered for two weeks, which is why the runs after the first also found nothing. Misses from before this version are forgotten, and a new miss is remembered for a week.

### 🗑️ Calibre's recycle bin
- **It opens without the long wait.** The bin is read in the background, eight books at a time, with a count of how far it got; you can leave the page while it reads. The result is kept for an hour (or until something is removed or put back), so opening it again is instant. **Read the bin again** reads it fresh.
- **Who removed what.** NovelCheck now keeps a log of every book it removes from Calibre: when, who, and from which page (Duplicates, Delete requests, Library cleanup). Each book in the bin says whether NovelCheck removed it and who did, and the full log is at the bottom of the bin view. Books removed before this version, or by Calibre or Calibre-Web, show as not removed through NovelCheck.

### 📦 Box sets
- **"Find what's inside" reads the box set's own table of contents first.** Most box sets list their books there, so sets a small AI doesn't know (like a lesser-known series) now fill in by themselves. The AI is only asked when the contents don't list the books.

### 🩺 Check my servers
- **The Audiobookshelf token line is right.** It asked Audiobookshelf in a way it doesn't answer and reported "404" even when the token works.

## [1.71.0]

### 🎨 A new look, on one standard
- **NovelCheck now follows Material Design 3**, the design guide Android apps use. Buttons, text fields, cards, chips, tabs, lists and pop-up windows are each defined once and look the same everywhere. This release brings the shared pieces, the top bar, navigation and Reading stats; the other pages pick up the new buttons, cards and fields now and get their own pass in a later release.
- **Navigation.** On a computer or tablet the pages are listed down the left side. On a phone the bottom bar shows the open page with a pill behind its icon. Icons are Material's own, stored inside the app.
- **Light theme, OpenDyslexic and Reduce motion** work as before.

### 🔎 Search at the top
- **Find a setting, an admin tool, a page or an action by name.** Press the search bar (or `/`, or Ctrl+K), type a few letters and press Enter: "password", "recycle", "duplicates", "theme", "sign out". It shows only what you may open. Books are still searched on the Library page.

### 📊 Reading stats, one page
- **Highlights, Books, Calendar, Records and Trophies are tabs of one page.** The separate Wrapped page is gone; old links open Records.
- **Records:** a tile each for this week, this month, this year and all time, with how it compares with the one before. The month and year tiles have **Play it as Wrapped**. Your awards for a year and your personal bests are lists.
- **Trophies:** how many levels you have earned out of all of them, **Next up** with a ring for each trophy you are closest to and a likely date at your recent pace, then every trophy with its named levels ticked off.
- **More to earn:** trophies for reading days, books opened, weeks in a row, weekend reading, long sittings, different authors, series started, non-fiction, re-reads, Hardcover goals, and (from an imported list) paper books, audiobooks and e-books. Four more yearly awards: day of the year, sitting of the year, most highlighted, streak of the year.

### 📥 Goodreads and other lists count in Reading stats
- **A list's "read" shelf now brings the day you finished each book, its pages, your stars, the format and how many times you read it.** Those books count in the year you finished them: in the tiles, records, awards, trophies and the Profile summary. Your stars become your rating here when you had not rated the book.
- **No KOReader needed to see your reading.** Someone whose reading comes only from a list gets Highlights (books, pages, authors, ratings, books by month, formats, genres, book length, every book), Records and Trophies. Hours, pages a day, streaks, the calendar and Wrapped still need KOReader, and the page says so.
- **If you imported a list before this version, import the same file once more**: the earlier import did not keep the dates.

### 📷 Check a book
- **A photo or screenshot is no longer taken at the AI's word.** What the AI reads has to be a real book by that author (one you have, or one the book catalogue knows). If it is not, the next AI reads the picture too, and the answer is accepted when one is confirmed or two agree. When nothing can be confirmed, you pick from the closest real books.
- **Screenshots are read better:** the AI is told to ignore series names, slogans, review quotes and a shop's buttons, and never to guess from the cover art.

### 🧬 Deep Scan
- **Why a book got its level, at the top.** A Deep Scanned book's window now starts with "Why Level 4": what that level means, how many of the noted parts reached it, and those parts by name with **Read it** beside each. A book gets the highest level found anywhere in it, so those parts are the reason. 
- **No spoilers in the lists.** Each part says what kind of content it is ("Kissing and romantic affection", "Sex on the page, described") with no names and nothing about the story. What actually happens is shown only when you open that part in the book.
- **Each part's level is easy to see while scrolling.** In "What the AI found", every part now starts with a coloured square holding its level (grey 0, green 1, teal 2, amber 3, red 4, deep red 5), with the part's name beside it and the level's name under it. A line at the top counts the parts at each level, highest first.

### 📦 Box sets
- **"Ask the AI what's inside" asks once.** The buttons could come back to life while an answer was on its way, so a set was asked two or three times and ended with "not found" after a long wait. Now each set is asked once, only its own card changes when the answer comes (titles you are typing elsewhere stay), and a set you already decided answers at once.

## [1.70.0]

### 🗑️ Calibre's recycle bin, in NovelCheck
- **See what was removed from Calibre, and when.** Admin → Library cleanup → "Calibre's recycle bin" lists every book Calibre still holds in its bin, whoever removed it (NovelCheck, Calibre or Calibre-Web): the title and authors it had, its files and sizes, and the day and time it was removed, with a count per day and a box to find a title.
- **Put back.** One button adds a book to Calibre again from its files in the bin, as a new entry with the title and authors it had. Calibre empties its bin by itself after a while, so this only works until then.

## [1.69.1]

### 🛡️ Duplicates in Calibre
- **The last copy of a book can no longer be removed.** NovelCheck's list can be a few seconds behind Calibre after a removal. Before removing anything, NovelCheck now asks Calibre itself which copies it still has, and refuses when that would take the last one. The list then refreshes by itself.

### 📱 Phones
- **Pulling down at the top of a page no longer reloads the whole app.** On Android that gesture reloaded NovelCheck, which is what "the page completely refreshes" was: it also cut off whatever was being saved, with a "Can't reach NovelCheck" message. The app's own Reload bar still appears when there is a new version.

### ⚙️ Admin
- **"Spent to date" explains itself:** it now shows what was spent in the last 30 days and says that your own AI counts as free (enter its watts under Machines & costs to count electricity). The total only moves when a paid AI service is used.
- **Saving settings is one write instead of sixty**, and is tried again when the database is busy, so it no longer fails with "internal error" during a long job.
- **Server errors in the bug report show their real cause** again; some only said "internal error".
- "Apply ticked to Calibre" can't be pressed twice.

## [1.69.0]

### 🏅 Records & awards (a new tab in Wrapped)
- **Your yearly awards**, for any year you have reading in: Book of the year, Your favourite (from your star ratings), Couldn't put it down, The doorstop, Author, Genre, Series and Month of the year, and a Night owl or Early bird award.
- **About twenty records, for all time or one year:** time read, longest day, most pages in a day, longest sitting, longest streak (and the one you are on), biggest week and month, most books finished in a month, fastest pace, fastest finish and longest haul, longest book, the book you spent the most time on, latest night and earliest start, your reading hour and weekday, most-read author, genre and series, and the day your statistics begin.
- **The year story now plays your awards and records too** (about 18 slides for a year with reading in it).
- Wrapped now has three tabs: 🎁 Wrapped (the story), 🏅 Records & awards, 🏆 Trophies.

### 👯 Duplicates in Calibre: something to compare
- **"Compare what Calibre has for each"** on every book: under each copy, Calibre's own title, authors, series, ISBN, tags and the start of the description, with what differs from the copy you keep marked, and a link to open the copy in Calibre-Web when its address is saved.
- **An empty Calibre entry (no book file in it) no longer carries a warning.** There is nothing in it to lose, so it counts as safe to remove.
- **The guided cleanup sends you to this page for extra copies** instead of a list of names and sizes, which was not enough to decide by.

### 🤖 When an AI service runs out of free requests
- **A service that says its daily limit is used up is left alone until it resets**, instead of being asked again on every book. In place of the service's long error you see one sentence: which model, and when NovelCheck will ask it again. Admin → Integrations shows "Daily limit used" on that AI.

## [1.68.0]

### 🔌 Integrations (replaces System checks)
- **One page for everything NovelCheck is made of and talks to**, under Admin → Integrations. Each row shows a status badge, what it is for, when it was last used, and a button where there is something to do.
- **NovelCheck:** your version and whether a newer one is out, the database (size, and "Check it" runs SQLite's own check of the file), the bundled Cloudflare connector with its version and whether it is connected, and the KOReader plugin version with what each e-reader last reported.
- **Your AI:** each AI and its models, whether the Deep Scan machine is answering, and whether newer versions of your Ollama models are out.
- **Your library and accounts:** the Calibre folder, the Content server, Calibre-Web, Audiobookshelf (with its last read), Hardcover, Libby and email, each with "Read now" where that applies.
- **Book data from outside:** Open Library, Google Books, the Discover lists ("Update now") and the cover finder ("Find now").
- **"Check now"** tests every connection, as "Check everything" did, and shows each result on the row it belongs to. It runs in the background, so you can leave the page. "Diagnose with AI" is still here, folded away at the bottom.

### 📱 Switching apps
- **No more "Can't reach NovelCheck right now" after switching to another app and back.** The phone pauses the app while it is in the background, and what it was loading failed. Those loads are now tried again quietly once the app is in front.

### 🤖 Models, in NovelCheck's terms
- **Every model in the Ollama list now says what it is good for here:** rating a book from its description, Deep Scan (reading the whole book) and reading cover photos. A small model on a Deep Scan machine shows "too small, tends to over-rate", and a "thinking" model is flagged as slow and prone to breaking the answer format.
- **A tip when a better-suited model is further down your list**, with "Put the best suited first". The list knows which job it is for (your AI rates; the Deep Scan machine reads whole books).
- **The models to download are described the same way**, and the GPU labels now say "Recommended on your GPU" and "Largest that fits (most careful, slowest)" instead of "most powerful". These notes are judged from each model's kind and size; they are not measured on your books.

### 🧹 Library cleanup opens at once
- **The summary is kept in memory** while the library is unchanged, instead of being worked out again (a second or two) each time. The page also shows the last numbers straight away and checks them behind the scenes.

### 📊 Reading stats
- **Wrapped and the trophies are back within reach:** a "🎁 Wrapped & trophies" tab on Reading stats. The link had gone missing once the page finished loading.
- **"Young Adult" instead of "Ya"** in the genre chart, the share card and Wrapped (and proper names for the other genres).

### 📚 Hardcover
- **Sync no longer fails with "UNIQUE constraint failed"** when a Want to read book is already on your Nightstand.

## [1.67.0]

### 👯 Duplicates in Calibre, easier
- **Pick the copy to keep.** Each book shows its copies side by side with "Keep this one"; the suggested copy is already chosen and outlined, with a line saying why. The others are what gets removed. No more ticking what to delete.
- **Each copy shows its own cover, its formats with file sizes, and when it last changed in Calibre**, so two copies can be told apart at a glance.
- **Sure ones first.** Books whose copies look the same are grouped at the top with one "Remove their extra copies" bar at the bottom of the screen. Books where something differs (size, a PDF or audiobook of its own) sit under "Check these" and are never removed in one go.
- **A "Remove the other copy" button on every book.** It answers at once and the book leaves the list; Calibre is read again in the background. For a book under "Check these" it asks first and shows the reason.
- "Books listed twice in NovelCheck" (a different job) is folded away on this page and opens by itself when there are pairs to look at.

### 🧹 Library cleanup
- **Removing extra copies no longer stops on a short title with a subtitle.** "Rant" in Calibre against "Rant: An Oral Biography of Buster Casey" in NovelCheck was still taken for the wrong library, because very short titles were not compared that way. A title followed by a subtitle or a note in brackets now counts as the same book, whatever its length.
- **One renamed book no longer holds up the rest.** A copy whose title really differs in Calibre is left alone, with a 🔔 notice saying which, and the other ticked copies are removed. It still stops with nothing removed when more than one in five differ, which means the wrong library.

## [1.66.2]

### 🐞 The link to recent bug reports shows more, and can tidy up after itself
- **Problems and ideas your family sent** (🐞 Report a problem or idea) are now listed in the link, each with its number.
- **"Diagnose with AI" results are kept with the bug reports**, so they show in the link and under Settings → Developer. The "Open GitHub issue" button, which did not work, is gone.
- **Handled things can be deleted through the link, one by one:** single server errors by their code, family reports (marked done), the debug recording, and reports and diagnoses by number. Before, the link could only empty the whole error list.

## [1.66.1]

### 🧹 Library cleanup
- **"Find covers now" (and Stop) no longer redraws the whole tab.** Only the list of tools updates, so the page keeps its place and open sections stay open. The list also updates after Tidy, Fill in now, Check and split and Read now, so the counts and buttons show what just happened.

### 🐞 Bug reports
- **The report now shows when part of a page is drawn again by itself**, which part, and how far down the page was ("redraw" lines), so a page that "refreshes" can be traced to the tap before it.
- **It also lists what was done, not only what failed:** every change sent to the server (join, remove, convert and so on) with its result.

## [1.66.0]

### 🤖 Admin → AI & Scans, tidied up
- **Six plain cards instead of a wall of settings:** **Your AI**, **Backup AI**, **Deep Scans**, **When it runs**, **Extras** and **Machines & costs**. Every setting is still there, in the card it belongs to. A closed card shows what it is set to (for example the model in use).
- **Rarely used settings sit under "Advanced"** in each card (fallback models, JSON mode, time limit, context size, token prices).
- **A Save bar appears as soon as you change something** and goes away once saved, on every Admin settings tab. No more hunting for the button at the bottom.
- **Notices show on the tab they belong to.** AI & Scans only shows AI notices. What waits in Library cleanup (delete requests, wrong covers, titles to tidy, box sets) and in System (problem reports) shows as a count on that tab.
- **Two main buttons**, "Rate waiting books now" and "Check Calibre for new books". Re-rating the whole library and emptying the queue moved under "More actions", each with a line saying what it does. The status reads in plain words ("The AI is idle", "Rating now: …").
- Your own AI filters are now in **Extras**; the photo-reading model is in **Your AI**.

### 📷 Check a book
- **The right book when two share a title.** A photo of *Part of Your World* by Liz Braswell showed the book of the same name by Abby Jimenez, because a lone title match ignored the author. A book by a clearly different author is no longer used.
- **Sideways and upside-down photos.** The photo is turned the way the phone was held before it is sent, and when the AI can see the photo but reads no title, it tries the photo turned each way.
- **"No image available" is not a cover.** Catalogues sometimes pass along a shop's placeholder picture as the cover. It is now recognized and dropped (ones already kept too), so the book gets a real or drawn cover.

### 🛡️ Library cleanup: safer about what it removes
- **Extra Calibre copies are checked before they are offered for removal.** Calibre duplicates are found by title and author, which can't tell a second copy from another edition or an omnibus. Each extra entry's files are now compared with the copy that stays. One that differs a lot in size, has a PDF or audiobook of its own, or can't be compared is marked with the reason and **left unticked**; "Tick all" skips those too. File sizes are shown for both copies.
- **"Remove other formats" only removes other e-book files** (MOBI, AZW3, KFX and the like). PDFs, comics, audiobooks and archives beside an EPUB always stay.
- **The EPUB that stays is checked first.** If it is missing, tiny or not a real EPUB file, that book is skipped and keeps all its files. A book with no EPUB is never touched, as before.
- **Books that would not convert are remembered, with the reason** (Formats… → "did not convert"). **Leave as is** takes a book off the convert list for good; **Offer again** puts it back. They keep all their files either way.
- **Leaving the page no longer stops the work.** Removing copies and tidying titles carry on when you switch tabs or close the app, and coming back to Library cleanup shows a job that is still running and follows it. Reading the Calibre library answers within a minute and carries on by itself, instead of failing after a long wait.

### 🧹 Library cleanup
- **Removing extra copies and formats no longer stops on a subtitle.** The safety check that keeps NovelCheck out of the wrong Calibre library stopped everything when Calibre's title carried a subtitle ("A Kiss of Shadows: A Merry Gentry Novel" against "A Kiss of Shadows"). Titles are now compared with that in mind, in Remove copies, Formats and Tidy.
- **The format job leaves a mismatched book alone and carries on** (it is listed with the reason). It only stops when more than one book in five doesn't match, which really is the wrong library. While it checks the books with Calibre at the start, it says so.
- **Filling in genres with a small local AI works.** Lists of 25 books were too long for small models and their answers were cut off ("invalid JSON"). A local AI now gets 8 books at a time, and a cut-off answer makes NovelCheck ask about fewer books instead of stopping.

## [1.65.0]

### 🤖 AI
- **Gemma 3 4B is in the model list** (Admin → AI → your Ollama): about 3.3 GB, and it can read cover photos. If a model that can see (Gemma 3, LLaVA and similar) is among your local models, Check a book sends photos to it first, even without filling in the photo-reading model.

### 📚 Calibre
- **Covers from Hardcover** in "Fix Calibre details from Hardcover": a Calibre book with no cover gets Hardcover's cover offered as a change to tick. Books that already have a cover are left alone.
- **Tidy's "the Content server has no book" message now says the likely cause:** books added with Calibre-Web aren't noticed by the Calibre Content server until the Calibre app is restarted.

## [1.64.0]

### 📷 Reading a cover photo, faster and more dependable
- **A photo-reading model for your own AI** (Admin → AI → Rate Caps & Batching → "Photo-reading model on your local AI"). Your local models (llama3.2, qwen2.5:3b) can't read pictures at all, so every photo waited on the cloud AI and failed when it was busy. Put a model that can see on the AI machine (for example `ollama pull gemma3:4b`, about 3 GB) and type its name there: photos go to it first, with no waiting on a busy cloud service.
- **Models that can't read photos are asked once, then skipped** (until the server restarts), instead of being tried on every photo.
- **A photo attempt gives up after 75 seconds per model** (it used to be able to take ten minutes), and the photo is sent a little smaller (1024 px), which small models read faster.
- **The error says what to do** when no model can see: set the photo-reading model.

## [1.63.0]

### 🎁 Wrapped
- **Books finished elsewhere count too.** When Hardcover has the day you finished a book (paper, audio, another reader), Wrapped adds it to that month's and year's "Finished" count and covers, without counting a book twice.

### 📖 KOReader plugin 1.53.0
- **A shelf for the Bookshelf plugin.** If you use Bookshelf, add a shelf there and pick **NovelCheck books** under Source / grouping: it shows the books NovelCheck downloaded to the device, newest first. It needs no network, and if Bookshelf isn't installed nothing changes.

## [1.62.1]

### 🔒 Security and upkeep
- **Updated `golang.org/x/net` to 0.62.0** (fixes two known vulnerabilities) and `x/crypto` to 0.58.0.
- **Debug recording never keeps secrets from an address:** the token in an OPDS address and values named token, code, key or password are replaced before a line is stored (the recording is part of the bug-report link).
- **Kids' accounts can't search the libraries or add books through Libby search** (the buttons were already hidden; the server now refuses too).
- **The Libby cover pass-through doesn't follow redirects**, so it can only ever fetch from OverDrive's own picture addresses.
- **Check my servers can't hang** on a server that doesn't answer (10 second limit), and a recorded line with no usable time is stamped now rather than 1970.

## [1.62.0]

### ⚡ Speed
- **A few database connections instead of one.** NovelCheck answered one database request at a time, so one slow request held up every page (the minute-long waits in your bug report). Reads now go on while a write happens, and writes still queue (up to 30 s) rather than fail. If it ever misbehaves, set `NOVELCHECK_DB_CONNS=1` to go back to one.

### 📷 Check a book
- **Photos no longer time out.** A photo that takes the AI longer than the remote address allows (100 s, the "524" error) is now read in the background: the page asks for the answer every couple of seconds and shows it when it is ready.

### 🔌 Check my servers, and uploads
- **Settings → Developer → Check my servers** (admins): harmless read-only questions to Calibre, Audiobookshelf and Calibre-Web: library names, whether NovelCheck's books are in the library Calibre serves, which features each has, what the Audiobookshelf token may do. The answer is part of the server report, so a bug report link carries it.
- **Upload (experimental), Admin → Library cleanup:** e-books (epub, pdf, mobi, azw3…) go into Calibre through its Content server and are read back to check they arrived; an audiobook (its files, a title, author, library and folder) is streamed to Audiobookshelf. Nothing is sent until you confirm the list. Run Check my servers first. The remote address refuses more than 100 MB a time, so send big audiobooks from the home network.

### 🔧 Smaller
- **Importing a list** no longer offers paper-book shelves or Calibre as the place to put it (that was what the "paper-book library" error was about).

## [1.61.0]

### 📚 Fix the Calibre library from Hardcover (Admin → Library cleanup)
- **A background scan compares every Calibre book with Hardcover** (your own Hardcover, by ISBN, else by exact title and author) and lists what differs: a **missing series and its number** (or a wrong number), a **tidy title and author** ("Cinder (The Lunar Chronicles Book 1)" becomes "Cinder", "Meyer, Marissa" becomes "Marissa Meyer"), a **missing ISBN**, an **empty description**, and **genres Calibre doesn't have yet** (added to the ones it has).
- **Nothing changes until you tick it.** Apply writes the ticked changes to Calibre in the background (one book at a time; Calibre renames files, so it is slow) and **reads each book back** to check Calibre kept them; anything it didn't keep is listed. "Not these" remembers your choice, and a new scan doesn't suggest the same thing again.
- It only fills gaps and tidies how something is written; it never replaces one book's details with another's. Covers are not part of this yet.

## [1.60.0]

### 🔗 Hardcover, both ways
- **Want to read and Paused sync with Hardcover** (with "send my reading to Hardcover" on). Books on your Nightstand that you haven't started go to Hardcover as **Want to read**; a book you put aside with **⏸ Back to Nightstand** goes as **Paused**. In the other direction, a book that newly shows as Want to read at Hardcover joins your Nightstand (only books that are in your library, up to 25 a sync, and taking one off the Nightstand sticks), and one you pause at Hardcover goes back to your Nightstand.
- **Books without an ISBN are found by title and author** through Hardcover's search, accepting only an exact title and author match, so a look-alike is never written to. This also covers star ratings.
- **Your Hardcover reading goal in Wrapped:** the yearly "reading goal" slide shows how far you are.
- See `docs/HARDCOVER_API.md` for what Hardcover offers that is still not used (reading journal, reviews, lists, finish dates).

## [1.59.1]

### 🔗 Hardcover, more dependable
- **Calls to Hardcover are spaced out** (it allows 60 a minute and a book sent takes up to five calls), wait and retry once if it says too many, say they come from NovelCheck, and **a write Hardcover refuses is now reported** instead of looking like it worked.
- **Checked against Hardcover's published schema:** ratings support half stars, and the calls we use are the ones the official KOReader plugin still uses. What else the API offers is listed in `docs/HARDCOVER_API.md`.

## [1.59.0]

### ⭐ Ratings and Hardcover
- **Your star ratings sync with Hardcover** (when Hardcover is connected and "send my reading to Hardcover" is on). A rating you give at Hardcover for a book you have read fills in here if you haven't rated it in NovelCheck; a rating you give here is sent to Hardcover for books already on your Hardcover shelves (as the nearest half star, which is what Hardcover uses). Yours in NovelCheck is never replaced by a sync.

## [1.58.0]

### ⭐ Your ratings
- **Rate a book with stars** in its window (tap a star; tap it again to clear; ¼ buttons fine-tune to 4.25 and so on). Wrapped shows your average and a breakdown, and new trophies count books rated and five-star books. Ratings are kept in NovelCheck and are only yours.

### 🎁 Wrapped, more
- **Top authors** (top four), **book extremes** (longest and shortest you finished), **currently reading**, and a big-numbers row on the last slide (books, pages, average rating).

### 🏆 More trophies
- **Habit:** one sitting (finished the day you started), lunch-break reader, streaks of 7, 14, 30, 60 and 100 days. **Book shape:** hot streak (books in a month), doorstops (800+ pages), quick reads (under 100). **Series and year:** series marathon, beat last year, and a yearly readathon (12, 24, 52, 100 books).

### 🔧 Smaller
- **A same-books join that fails** says the pair is out of date (a book was joined or removed meanwhile) and refreshes the list, instead of "That didn't work".
- "A evening reader" is now "An evening reader".

## [1.57.1]

### 🔄 Conversion, Discover and Libby search
- **Conversions that failed with "Could not find reasonable point at which to split"** (Cat's Cradle, Misery, Othello, The Terror…) are tried again without calibre's small-file splitting, which is what that error is about. The failure message says so if the second try fails too.
- **Libby search results match what you typed.** A search for "Cinder" no longer fills the list with other popular books: results must contain the words you typed (title, author or series), best matches first; if nothing does, it says so.
- **Libby search covers show.** The page only shows pictures from NovelCheck itself, so covers now come through it (only OverDrive's own picture addresses are fetched).

### 🐞 Fixes from the bug report
- **Nightstand** no longer throws a "Sortable" error when its list is empty (the page's reorder code needs a list on screen).
- **Stats page** no longer fails with "localeCompare is not a function" when switching periods (a number used as a label).

### 🛏️ Nightstand
- **⏸ Back to Nightstand** on a book you are reading: puts it at the top of your want-to-read list without removing it. Your place in the book is kept, so starting it again carries on. (Remove still takes it off the Nightstand altogether.)

### ⚡ Speed
- **Fixed the slowdown while Debug recording was on, and a source of slowness without it.** NovelCheck answers one database request at a time, and two things stacked up behind each other: recording wrote to the database inside every request (it now keeps lines in memory and writes them together once a second), and pages that check for news every few seconds (the AI activity pill, the Admin and Usage pages, Remote access) asked again before the last answer had come back (they now wait for it).
- **Debug recording also keeps what the app showed you** (messages such as "3 tidied" or an error), so a timeline says how an action ended.

## [1.57.0]

### ⏺ Debug recording (Settings → Developer, admins)
- **Record what happens for a few minutes** (5, 15 or 60; it stops by itself after an hour at most). While it is on, **everyone's NovelCheck shows a red bar** saying an admin is recording to fix a bug, and the server keeps a timeline: pages opened, buttons tapped (by name), what was typed (never passwords, codes or keys), failed requests and errors from each browser, plus **every call the server answers**, including KOReader, OPDS and sync.
- **Timeline in Settings → Developer** to refresh, copy or delete, and the latest 200 lines are **added to bug reports** (the server report). Lines are deleted after a week.

### 📱 Phone layout
- **Library search gets its own full-width row** on a phone, with Sort, Filters and ⋯ under it (the box had shrunk to a small square).
- **Seasons and Collections chips wrap** instead of scrolling sideways.

## [1.56.2]

### 🧹 Tidy in Calibre
- **Tidy finds the right Content-server library by itself.** When the book numbers NovelCheck knows aren't in the library the Content server is serving (the "has no book #1690" message, and "0 tidied"), Tidy now asks every library the server offers, picks the one whose titles match, remembers it, and tidies there. If none match, the message lists each library it checked and how many books matched.

## [1.56.1]

### 🔄 Update bar
- **"A new version of NovelCheck is ready" bar with a Reload button** at the bottom of the screen. The app now checks every couple of minutes while it is open (and soon after opening), instead of reloading by itself after five minutes away. Nothing reloads while you are typing; tap ✕ to wait.

### 🗂 Series
- **Series gaps:** each missing number on a series page has **🔎 Find it**, which searches your Libby libraries for that book and offers **Add to Nightstand**. Books you don't have also get **＋ Nightstand** next to Request. Novellas and half-numbered books (1.5) are labelled "Novella, not a full novel".


## [1.56.0]

### 🔎 Search your libraries
- **Search Libby for any book, even one NovelCheck doesn't have** (Discover → "Search my libraries", and in the Library when a search finds nothing): looks at every library you added, e-books and audiobooks, and shows the best answer for each book (available now, or the holds and wait) with a link to it. **➕ Add to Nightstand** makes the book and puts it on your Nightstand.

### 🎁 Wrapped, full screen
- **Wrapped is now a full-screen story** (tap ▶ Play my Wrapped): a gradient slide at a time with progress bars across the top, huge numbers, your most-read book, top author with the runner-up, your reading personality, a 24-hour "A day in the life" donut, your longest day, an achievements grid, and a last slide to share a picture. Tap the right side to go on, the left to go back; Esc or ✕ leaves.

### 🧰 Filters and fixes
- **The Library filter panel is a compact popover** over the results (a full sheet on a phone) with a **Done** button; the rarely used hide-content options sit in a "More" fold. It closes when you tap outside it, and it no longer pops open by itself.
- **Fixed Settings closing on an iPhone.** A window closing at the same moment as a link going to another page could send the browser back a step and undo the trip. Windows now only step back when nothing else has changed, and a tapped link is no longer mistaken for the Back button.

## [1.55.0]

### 🎁 Wrapped, trophies and records
- **Wrapped, by month and by year** (Stats → 🎁 Wrapped & 🏆 trophies): your reading as a short story you tap through: hours and pages, books finished, your most-read book, your top author and genre, when you read, your longest day, and the awards you earned (Night Owl, Early Bird, Marathon Reader, Series Binger, Genre Explorer, Page Turner, Steady Reader, Weekend Warrior, Finisher). The last slide makes a picture to save or share.
- **Trophies with levels** (bronze to diamond): books finished (1 to 250), pages, hours, streaks, marathon days, night owl and early bird days, genres tried, whole series finished, books by one author, the longest book, highlights, and a trophy per genre (Young Adult, Romance, Science Fiction, Fantasy, Mystery… 10, 25, 50, 100).
- **Records:** your longest reading day, most pages in a day, longest streak, the book you read the most pages of, the longest book you finished.
- Worked out in your browser from your KOReader statistics, in your own time zone; nothing is shared unless you share the picture.

### 📚 Libby: more than one library, and audiobooks for every book
- **Add several libraries** (Settings → Connections). Each book is checked at all of them and the one with a copy available soonest is shown; the book window lists the libraries that have it.
- **Every book you don't have as an audiobook is checked at your libraries in the background** (every few days, slowly) and the answer is kept: the Library shows 🎧 on the card, and the filter **At my Libby library** shows only books whose audiobook is there, or there and available now. **Check my books now** starts it at once.
- **Discover filters:** 📚 Lends it, 🎧 Audiobook, ✓ Available now.

### 🗂 The Series page fills in what's missing
- **🔎 Find the missing books** looks the series up at Hardcover and adds the books you don't have in their place (including ones beyond your highest number): each shows its cover, **⭐ Request it**, and whether your library has it (📚 and 🎧). "Not in your libraries" gaps become real books.

### 🖼️ More covers, and a cleaner Library
- **Cover finder:** books Calibre has no cover for get a real one from Open Library or Google Books (once a day in the background, or **Find covers now** in Admin → Library cleanup), only when the title and author really match.
- **The Library's filters are a clean bar:** one search box and Sort, a **Filters** button that opens a labelled panel, a chip for each filter that is on (tap × to remove it), a styled picker for authors and series in place of the browser's white list, and a **⋯** menu for the rare actions.
- **The Admin tab bar no longer jumps** between pages.
- **Faster pages:** Discover is kept for 15 minutes, Suggested Reads on the Nightstand (about 4 seconds to work out) and the Admin page's AI check (about 3 seconds) for a short while too. A change you make in the app clears them at once.

### 🔄 In step with the new KOReader
- **KOReader 2026.07's "Send document metadata":** NovelCheck uses the title and author it sends with progress to find books it never sent to the reader, and the plugin (1.52.0) turns it on. The self-test now also reports your progress-sync, statistics-sync and Cloud storage set-up.
- **Conversions run in the background.** An admin banner shows progress on every Admin page and you get a notice when they finish; the guided cleanup stops after starting them and you review the next steps when they are done.
- **Currently reading, twice:** "Series: Title" next to "Title" (like the Last Days of Lorien) is now found as a pair.
- **"X% shared wording"** now says what it means, and each pair has the full description to read.

### 🔍 Library cleanup: review first, convert first
- **Every cleanup step shows a review before it changes anything.** A list of exactly what will change (the titles, the books to convert, the extra copies, the formats), each with a tick box to leave it out. Nothing happens until you confirm, and you can stop at any step.
- **Converting comes before removing copies.** The steps now run: tidy titles, convert to EPUB, remove extra Calibre copies (each book keeps its best one), remove other formats, re-read Calibre. A book is never judged on a file that is about to get a better one.
- **"0 tidied" now says why.** When a title can't be saved, the window names it and the reason. One reason it found for you: the Content server already shows the tidy title while the library NovelCheck reads still has the old one, which means they are two different copies of the library.
- **The bug report explains conversions and tidying.** It lists the last format job with the reason each conversion failed, and the titles waiting to be tidied with what the Content server has for each.

### 👯 Joining books: you choose which one stays
- **Join, keeping either one.** Each pair has a "Join, keep this one" button on both books; the joined book shows that one's title and details (for example a newer edition), and everything moves onto it. The rating always stays with the better rated of the two.
- **A suggestion and the facts to decide.** Covers, when each was added to the library (Calibre's date), when its file last changed in Calibre, file size, how many Nightstands it is on, and a suggested one to keep and why (in a library; newer in Calibre; more used). **Click a title to open the book** and look at it.
- **Fewer wrong pairs.** "Both #1" of two different series (a novel and a Lost Files novella), a much shorter file, or one calling itself a novella or short story now count against a pair and put it under "check carefully".

### 📦 Box sets
- **A box set's inside titles join books you already have.** "Sea of Monsters" for "The Sea of Monsters" used to make a second copy; it now joins the book you have. The window shows, for each title, "already in your library as …" or "new book", checks as you type, and asks before splitting. The box set's title opens its book window, with its formats and when it was added.
- Titles with HTML entities ("The Queen &amp; The Assassin") now show correctly.

## [1.54.0]

### ⬅️ The Back button takes you back to where you were
- **Tabs are steps you can go back to.** Switching tabs in Admin, Settings or Deep Scan used to replace the page in the browser's history, so Back skipped the tab you had just left. Each tab you choose is now a step.
- **Back closes an open window.** A book's window (or any other window) no longer makes Back leave the page behind it; Back, or a phone's back swipe, closes the window first. Closing it with its own button leaves no extra step behind.
- **Back returns to the same place.** The page opens where you had scrolled to, and the Library keeps your search and filters when you come back from a book's series, an author's page and so on. Opening a page by a link still starts at the top.

## [1.53.0]

### 🧹 Admin → Library cleanup
- **A tab of its own in Admin** with everything that keeps the library tidy: the guided cleanup (tidy titles, remove extra copies without an EPUB, convert to EPUB, remove other formats, re-read Calibre) and, below it, each tool on its own with how much is waiting: books listed twice (with the evidence), duplicates in Calibre, tidy titles, formats, genres, box sets, covers reported wrong, delete requests, reading the Calibre library, and your Kindle matching.

## [1.52.0]

### 📚 Finished books leave the Nightstand, and genres fill in by themselves
- **A book you finish on the Kindle moves to finished.** The KOReader plugin (1.51.0) now reports KOReader's own "finished" mark, so a book that ends with excerpts and back matter (the last page is often under 97%) no longer stays in "Currently reading". It only moves books already on your Nightstand, and not one you put back to read again.
- **Genres fill in by themselves.** New books Calibre has no tags for get their genres automatically (every ten minutes when needed, same switch and hours as automatic rating, within the token cap). The Fill in with AI button is still there for a push. They were always saved; they just only ran when you pressed the button.

### 🛡️ Joining books: evidence first, fewer wrong joins
- **A book and its spin-off are no longer treated as the same book.** "I Am Number Four" next to "I Am Number Four: The Lost Files: Six's Legacy", or two spin-offs of the same book, used to be offered as pairs.
- **Every pair shows its evidence.** Both books side by side with series and number, ISBN and a piece of the description, a badge (✅ very likely, 🟡 probably, 🔴 check carefully) and the reasons: same author, same ISBN, same place in a series, descriptions that say the same thing, one title with an extra subtitle. Doubtful pairs sit in a folded section at the bottom and ask "Join anyway?" before joining.
- **Clear contradictions take a pair off the list:** a different number in the same series, or descriptions about different things.
- **Automatic joining is stricter.** After a sync it only joins a looked-up or wished-for entry with its library copy when they have the same ISBN or the same title once tidied (by the same or an unknown author), and the notice now names the books it joined.
- The Nightstand banner shows the same evidence.

### 🏷️ Saving a title in Calibre no longer hangs
- **It answers in a couple of seconds.** Saving a title (or tidying many) used to wait for a full re-read of the Calibre library, about 15 seconds each. The re-read now happens in the background, and several saves close together share one.
- **One save at a time, and the button says so.** The button turns into "Saving in Calibre…" and stays off until it's done; pressing it again used to queue more re-reads (five presses took over a minute).

## [1.51.0]

### 🧹 Library cleanup, one flow
- **Admin → Delivery & Services → Calibre Library → 🧹 Library cleanup…** shows what is left to tidy and runs the steps in order: tidy titles in Calibre, remove extra Calibre copies that have no EPUB (only where another entry of the same book has one), convert the rest to EPUB, remove the other formats where an EPUB now exists, then re-read the library. Duplicates in NovelCheck itself (joining books) stay a step you do by hand first. Before anything is deleted you see what and how much; the copies and files go to Calibre's recycle bin, formats can be undone for 7 days under **Formats…**, and a book that can't be converted keeps its original file. It ends with the Kindle steps (Update plugin, Report books, Tidy book names, Download all Nightstand).
- **Nightstand: "👯 look like the same book".** When two books on your Nightstand look like one (a title written two ways), a banner offers **Join them** or **Not the same**. Admins and editors see it.

### ⚡ Faster tabs
- **The heavy lists are kept in memory** for 45 seconds: Library, its filters, Collections, Discover and reading stats. Anything you change in the app clears them at once, so a page never shows old data after your own change.
- **A "Slow requests" section in the bug report** lists the pages that took over 0.8 seconds (how often, average and worst time), so a slow tab can be traced to its cause.

## [1.50.0]

### 🐞 Bug reports you can trust
- **Server errors with a code.** Every failed request is kept with its real cause (Admin → Settings → Developer → ⚠️ Server errors, and in the report). When the app shows an error with a code like `E-3f2a`, that line is in the list. Before, a failure only said "could not save" and the cause was lost.
- **Reports send themselves.** An error in the web app, a server error, or an error in the Kindle plugin sends a report by itself (at most once every few minutes, never what you type; a switch in Settings → Developer and in the plugin's Developer tools turns it off).
- **Say what you were doing.** A note box beside **Send report**, and **Developer tools → Send log with a note…** on the Kindle.
- **Clearer reports.** Everything is in UTC, and the self-test adds the folders the plugin uses, the free space and the time.
- **Delete reports once they're dealt with.** **Delete all** in the Bug reports list, and the secret link can delete reports by number (`…/resolve?ids=3,4&errors=1`). Reports also expire after 30 days.
- **The link switches itself off after 7 days.** It shows when it was last opened and can be extended or replaced.
- **Plugin 1.50.0.**

## [1.49.0]

### ➕ Add books from your e-reader to NovelCheck
- **Match books on my e-readers** now has **➕ Add to NovelCheck**. A book on your Kindle that isn't in any library (a sideloaded series, something KOReader's statistics saw) becomes a book in NovelCheck under its tidy title and author, so it can be rated, counted in your stats and found again. An author of "N/A" or "Unknown" is left blank, and a book NovelCheck already knows is reused instead of doubled. Added books leave the list for good. **Match…** and **Not a book** work as before.
- **KOReader plugin 1.49.0:** books from your Nightstand are saved in a **NovelCheck** folder inside KOReader's home folder (where your other books are), and the message after **Download all Nightstand** names the folder. Scans for "On this device" and Tidy book names cover the whole home folder.
- **Bug reports you can find, and a link to share them.** Settings → Developer now has a **Bug reports** list (admins) holding what the Kindle plugin and the browsers sent, and **📤 Send report to NovelCheck** in the browser card. **Run self-test** in the plugin sends its result to NovelCheck by itself. Admins can make a **secret link** (Settings → Developer → Link to recent bug reports) that shows the server report and the latest reports as plain text with no sign-in; make a new one to replace it, or turn it off. Sync codes are blanked out of every log, and the link's secret never appears in its own report.
- **Fixed: "Report books on this device" could fail with a server error (500).** A Kindle book you had matched or added to a book that was later merged into another (for example by the new automatic duplicate joining) left a stale link, and saving the report broke on it. Merging now carries those choices over, and a stale one is ignored.
- **Fixed (plugin): "New in library" and search could be missing** with "attempt to call method 'newInLibrary' (a nil value)". The plugin now loads its own files by path, so a KOReader module with the same plain name can't take their place, and a file that returns the wrong thing is recorded in `LOAD_ERROR.txt`.

## [1.48.0]

### 🎧 Audiobooks: Audiobookshelf, download, listen, Libby
- **Connect Audiobookshelf** (Admin → Delivery & Services): its address and an API token, once. NovelCheck reads your audiobook libraries (title, author, **narrator**, series, **length**, cover, genres) every few hours, or press **Read the library now**. A title you already have as an e-book becomes **one book with two formats**.
- **Peppers on audiobooks:** they're rated like every other book, so Prologue's missing spice ratings are here. Kids' content rules apply to audiobooks too.
- **Sort and filter by length:** Library has **Longest / Shortest audiobooks**, a **length** filter (under 5 hours … over 20), a **narrator** filter and an **Audiobooks** format. Cards show 🎧 length and 🎙 narrator.
- **Download from the browser:** **Download audiobook** (one file as it is, several as a zip) next to the e-book's Download.
- **Listen in the browser:** a player with speed, sleep timer (even "end of this chapter"), chapters, bookmarks and lock-screen controls. It keeps your place in Audiobookshelf, and **Continue listening** shows on Nightstand. The audio goes through NovelCheck, so your server's address and token never reach a browser.
- **Your own place:** Settings → Connections lets each person link their own Audiobookshelf account (the password isn't kept). Until then everyone shares the family account.
- **Ask for the other format:** on a book you have as an e-book only (or audiobook only, or on paper) press **Request the audiobook / e-book**. It goes on the wishlist with its format, and is marked got when it arrives.
- **Libby audiobooks:** a book's Libby box and the Discover chips also check the audiobook, with a link to borrow or hold it.

### 🛠 Developer tools, and fewer duplicate books
- **KOReader plugin 1.48.0 → NovelCheck → Developer tools:** *Run self-test* (connection, sign-in, the plugin's files and versions), *View log* (every menu tap, request and error), *Send log to NovelCheck*, a detail switch and *Clear log*. Each part of the plugin now loads on its own, so one broken file can no longer make the whole plugin vanish; the reason is kept in `LOAD_ERROR.txt` in the plugin folder.
- **Fixed:** a backslash in the server address or sync code is now escaped correctly in the plugin's pre-connected download (`config.lua`).
- **Admin → Settings → Developer:** the logs a Kindle sent (copy or delete them), a copy-able **server report**, and for everyone a record of **this browser** (pages, button names, failed requests, errors; never what you type) to copy and send when something doesn't work.
- **"Listed twice"** (Library → Find duplicates): books that look like the same book under different spellings or a missing author, with **Join them** or **Not the same**. A book that was only looked up or wished for (for example on a reading list before the e-book arrived) now **joins its library copy by itself** after each sync, bringing the Nightstand and wishlist entries with it.
- **Calibre-Web uploads show up sooner:** with a sync schedule on, NovelCheck notices when Calibre's database changes and syncs about half a minute later instead of waiting for the next scheduled one. The sync also counts entries it couldn't read.

## [1.47.0]

### 📤 Share your reading as a picture
- **A Share button on 📊 Stats** (next to the title) makes an image of the period you're looking at, like a year-in-review: your name and the period, **books, pages and reading time** in big numbers, your top genre and most read author, your streak, and a row of the **covers** you read.
- **Pick the look:** **Story** (1080 × 1920, for phone stories) or **Square** (1080 × 1080), three colour themes (Midnight, Sunrise, Forest), and a switch for the covers. The preview redraws as you choose.
- **Save image** downloads the PNG; **Share…** opens your phone's share sheet (on a browser that can't share files it saves the image instead).
- **It's drawn on your own device** from your stats and goes nowhere until you save or share it, so you decide what leaves. It's for your own stats only (not other people's profiles).

## [1.46.0]

### 👥 Share your reading with the family
- **A new Readers page** (👥 in the menu) lists the family members who share their reading profile, each with last 7 days, this month and books finished this year, and (if they choose) what they're reading right now. Tap someone for their full reading report: the same Report, Books and Calendar as your own stats, in the same clean sentences.
- **Private until you switch it on.** Settings → Account → **Sharing** has two switches: *Share my reading profile with the family*, and separately *Also show what I'm reading right now*. Nothing is shared by default, and someone whose profile isn't shared simply isn't there for others.
- **People only see what they're allowed to see.** A kid viewing a parent's profile only gets books their own content rules allow, never pepper levels, and "reading now" is hidden if the book isn't allowed. **Highlights, notes and files that aren't in your library are never shown** to anyone else.
- **Kids: a parent decides.** A kid can't turn sharing on. Parents get a **Kids' profiles** list in the same Sharing card to share each kid's profile (and show what they're reading now). Parents can always see their own kids' reading, shared or not, marked "not shared" on the Readers page.
- A new step in the How-to guide explains it.

## [1.45.2]

### 🔌 Remote access connector updated
- **The bundled Cloudflare connector (`cloudflared`) is updated to 2026.10.4** (from 2026.9.3), the one behind **Admin → Remote access**. Nothing about how remote access is set up changes. If you use it, check **Admin → Remote access** still shows connected after you update; if it doesn't, tell me.

## [1.45.1]

### 🔒 Security update
- **Updated `golang.org/x/net` to v0.60.0**, which fixes five known vulnerabilities in the HTTP/2 code NovelCheck's web server and outgoing requests use (found by `govulncheck`). Nothing else about how NovelCheck works changed.
- The automated tests now use the newest patch of the Go version, like the container build.

## [1.45.0]

### 🔍 Search and add books from the Kindle
- **NovelCheck → Search library** (plugin 1.45.0): type any words of a title or author (in any order) on KOReader's keyboard and get your NovelCheck library's matches. Each row shows the content level (L2) and whether the book is **on device** or already on your **Nightstand**.
- **Tap a book** for its page: title, author, series, **content rating with the reason**, the verdict and a summary, and the format and size, with two buttons: **Download to this device** and **Add to Nightstand**. A downloaded book is reported straight away, so it joins your Nightstand and Profile counts like any other.
- **NovelCheck → New in library** lists the newest books the same way.
- Only books with a file NovelCheck can send are offered, and kids' content rules apply exactly as everywhere else.
- **A backup address** (NovelCheck → Connect, fourth box): when the main address can't be reached (away from home, say), the plugin tries it. Type your home address there if the main one is the remote address, or the other way round.
- Update the plugin with **NovelCheck → Update plugin**.

## [1.44.0]

### 🔎 Finding a book, and matching books on your Kindle
- **Search finds books by any words, in any order.** The library search used to need your whole phrase to appear in one place, so "reptile snicket" or "nines legacy" found nothing. Now every word just has to be somewhere in the title, author, series or tags, apostrophes and punctuation don't matter, and case doesn't matter. This is what the Library search box and **Match…** use.
- **The Match window stays open when you click outside it** (only ✕ closes it), so a stray tap no longer throws away your place.

### 🧭 One look for tabs, and a clearer home for your password
- **Every tabbed page uses the same tab strip:** Admin, Settings, Reading stats, and Discover / Wishlist / Events. It's one underlined row that scrolls sideways on a phone and keeps the open tab in view (Admin's phone sheet and the other styles are gone).
- **Your password moved to Settings → Account**, next to family device and PIN. Settings now has four sections: General, Account, Kindle & KOReader and **Connections** (Hardcover and Libby, previously "Accounts"). Profile is just you.

## [1.43.0]

### 📊 Reading stats, rebuilt to read like a report
- **A report, not a dashboard.** Each card asks a question and answers it in a plain sentence: "You read 8 books", "You turned 2,309 pages", "You mostly read fiction", "Adventure is your top genre", "You read most in the evening, around 9 pm". Pick **Week, Month, Year or All time** and step back and forward through them (‹ ›). There is a **📊 Stats** entry in the menu now.
- **What's in the Report tab:** big numbers (books, pages, reading time, authors, days, finished); a row of the covers you read (tap one for its page); pages over time; reading time by day or month; your pace (and, for this year, how many books you're on track for); fiction vs non-fiction; top genres; what peppers you read; book length; your most read authors with their covers; series progress ("2 of 7 books"); when you read; and your top books by time. Cards for things NovelCheck doesn't know about a book (genre, series) simply leave those books out.
- **Books tab, like KOReader's lists:** your reading **by week** or **by month** with time and pages; tap one to see the books read then, and tap a book for **its own page**: time spent, days reading, average per day, start and last read dates, last page of the total, pages turned, average time per page, **highlights and notes**.
- **Calendar tab:** a month you can move through and the last year at a glance; tap a day for its hours and books.
- Highlights and notes now come across from KOReader's statistics too. The first time you open Reading stats after updating, NovelCheck re-reads your stored statistics file to pick them up.

## [1.42.0]

### ⚙ Profile and Settings, split
- **Profile is now who you are:** your name and role, **📊 My reading** (last 7 days, this month, your streak and books finished this year, with a button into Reading stats), your reading taste, your password, and the content rules on your account.
- **⚙ Settings is new, and holds everything you can change**, in three sections instead of one long page: **General** (appearance, start page, Start Reading delivery, family device and PIN, phone notifications, and adding books), **Kindle & KOReader** (KOReader setup, the plugin and your e-readers, reading sync) and **Accounts** (Hardcover and Libby). Each section has its own address, like `#/settings?s=kindle`, so it can be opened or shared directly.
- **Admin now comes after Settings** in the menu: Profile, Settings, Admin (on phones too, in the More sheet).
- Everything moved, nothing was removed, and the wording follows: "Profile → KOReader setup" is now "Settings → KOReader setup" in the app, the guide, the plugin and the docs. The Get started guide describes Profile and Settings.

## [1.41.1]

### 🛏️ Up Next is now Nightstand
- **Up Next is renamed Nightstand** everywhere: the menu, the page, the buttons ("＋ Nightstand", "Add to your Nightstand", "Hidden from your Nightstand"), the Kindle plugin's menu and the docs. The page's sections are **Currently reading** and **On the pile**. Nothing about how it works changed, and old links keep working.
- **Explained in Get started:** the How-to guide has a new **🛏️ Your Nightstand** step (what it is, how books get there, how to hide one), and the page shows a short note once, with **Got it** to dismiss it, for people who already know it as Up Next.
- **Check a book keeps every scan.** Scanning several covers in a row used to show only the last one (each new photo replaced the one before). Now each photo gets its own card, newest on top; they are read one at a time, and **Choose a photo** accepts several at once. A cover that can't be read says why under **Why?** (which AI failed and how, for example busy or text-only) and has **↻ Try again** and **✕ Dismiss**.
- **Photos try the cloud AI first.** A small local AI is usually text-only and would just fail on a cover, so the AI on the internet (such as Gemini) gets the first try, then the home one.
- Plugin 1.41.1 carries the new name (update from **NovelCheck → Update plugin**).

## [1.41.0]

### 🔎 Books on your Kindle find their match, and you can fix the rest
- **Every file in your library is fingerprinted** (KOReader's own file fingerprint), slowly in the background, so a book copied to the Kindle is recognised by its file whatever it is called. This is the exact way, and should catch most of your books.
- **The plugin sends each book's real title and author** (read from the book itself, a few more each time and remembered), instead of guessing from the file name. NovelCheck also reads file names better: `Author - Title`, `Title - Author`, underscores, ID suffixes and "(z-lib.org)" tags.
- **🔗 Match books on my e-readers** (Profile → KOReader plugin, and the "Match the unmatched" link under On your KOReader in Up Next): for each book NovelCheck couldn't match, search your library and pick it, or mark it **🚫 Not a book** (thumbnails, KOReader's own guide). Your choice is remembered by the file, so every later sync keeps it. A book you match joins Up Next like the others.
- Books still unmatched after all that are looked up in Open Library (a cover and real title), now for the files on your devices as well as the statistics.
- **Set up everything fixed after the first real try:** it no longer stops when it cannot see KOReader's Cloud storage plugin (it was wrong to ask you to turn it on), and no longer ends with a Restart button, which on a Kindle just closes KOReader. Progress sync is now changed in place when a book is open. Restart (reopen KOReader from KUAL) only if progress sync doesn't pick the settings up.
- **The plugin reports itself after an update:** Profile no longer keeps saying "plugin 1.39.0: update to 1.40.0" after you updated. A newly updated plugin tells NovelCheck its version at its next start (and when Wi-Fi connects), instead of waiting for the six-hour timer.
- **NovelCheck → About (plugin)** shows the version, your server, and the project's GitHub link with a button that shows it as a QR code (KOReader has no browser), so you can open it on your phone.
- **Plugin 1.41.0 → Tidy book names:** shows your library's title and author in KOReader for the books NovelCheck matched (it asks first, changes only the displayed name, never the files, and can be reset in KOReader's book info). Update the plugin from **NovelCheck → Update plugin**.

## [1.40.0]

### ⚡ One-tap setup in the KOReader plugin
- **NovelCheck → Set up everything** (plugin 1.40.0): checks the connection, points KOReader's **Progress sync** at NovelCheck (with auto sync on) and **Reading statistics → Cloud sync** at your NovelCheck folder, then asks to restart KOReader. No Cloud storage account, WebDAV address or folder picker by hand. A fresh install from Profile offers it the first time KOReader starts. The OPDS catalog is no longer needed (the plugin has Up Next and downloads).
- **Sync reading statistics now** (same menu) presses KOReader's Synchronize now. **Automatic → Sync reading statistics when online** does it when the Kindle connects (switched on by Set up everything).
- This writes the values KOReader would store itself; it is the part I could only check by reading KOReader's source, so try it first on a book you don't mind. If it can't finish it says what to do (for example turning on the Cloud storage plugin), and the manual steps still work. Update the plugin on the Kindle with **NovelCheck → Update plugin**.

## [1.39.1]

### 📊 Reading stats fills in from your existing sync
- **Reading stats was empty for statistics that synced before 1.37.** NovelCheck kept the book totals but never read the page-by-page reading, and KOReader only uploads again when something changes. NovelCheck now reads the statistics file it already has the first time you open Reading stats (and again at most every 10 minutes while it's empty).
- If your file has books but no page-by-page reading, the empty page now says so, with what to do (read a few pages, then press **Reading statistics → Synchronize now**).

## [1.39.0]

### 📖 Your Kindle's books in Up Next
- **Books on your e-reader join Up Next by themselves.** Each time the plugin reports the device, the ones NovelCheck recognises in your library are added: ones you've started as **Currently reading**, the rest queued at the end, finished ones skipped. A switch in Profile → 📱 KOReader plugin turns it off. (Books NovelCheck can't match to your library can't be added.)
- **Currently Reading** (top of Up Next) now says **▶ Currently reading** with how far you are, which device and when.
- **🙈 Hide** on any Up Next row or Currently Reading card takes it off the list without touching your library. **Hidden (n)** under the section brings any back. A hidden book is never re-added by the Kindle.

### 🔎 Books on your Kindle find their match
- **Smarter matching to your library:** "Hostile Hospital" now finds "The Hostile Hospital" (a leading The/A/An doesn't matter), "Snicket, Lemony" is the same author as "Lemony Snicket", and "The Lost Files: Six's Legacy" finds "Six's Legacy" (or the other way round). It only matches when exactly one book fits, so two books of the same title are left alone unless the author tells them apart. Books that didn't match before are checked again when you open Up Next, so your existing list improves without another sync.
- **Open Library for the rest:** a KOReader book that is truly not in your library is looked up in Open Library (free, at most 15 an hour), which gives it a cover and its real title and author. It is not added to your library (it stays "not in your library", like Check a book). Files that aren't books (KOReader's own guide, thumbnails, IDs) are skipped.

### 🔄 The plugin works by itself, and updates itself
- **Automatic (NovelCheck menu → Automatic):** when the Kindle gets online, at most every 6 hours, the plugin reports its books (on by default) and can download new Up Next books (off by default). Up Next stays right without opening any menu.
- **Update plugin (NovelCheck menu):** downloads the newest plugin files from NovelCheck, checks that they are valid, swaps them in (keeping the old files as `.bak`) and asks you to restart KOReader. When NovelCheck has a newer plugin, the plugin mentions it once after connecting. It never installs without asking.
- Profile → 📱 KOReader plugin still shows each device's plugin version.
- The plugin is parser-checked only; it has not been run on a real Kindle. The Lua is now split over several files (`updater.lua`, `devicereport.lua`, `devicebooks.lua`), so copy the whole folder when installing by USB.

## [1.38.0]

### 🧩 KOReader plugin: what's on my Kindle
- **NovelCheck → On this device** lists the books on the e-reader with how far you are in each (new, a percent, or finished) and their size. Tap one to **delete** it, after a confirmation. **Delete finished books...** clears them all at once, listing them first. NovelCheck never removes anything without asking on the device.
- **Download all Up Next** gets every Up Next book that isn't on the device yet in one go.
- After a delete or a bulk download, the plugin reports the device again, so Profile stays right.
- **NovelCheck → About** shows the plugin version.

### 📱 Manage the plugin in the app
- **Profile → 📱 KOReader plugin:** **Download the plugin** gives a zip already connected to your account (server, username and sync code inside), so nothing is typed on the Kindle: unzip, copy the `novelcheck.koplugin` folder into `koreader/plugins/`, restart KOReader. The same download is in the KOReader setup window, including for kids' devices (parents).
- The card lists each e-reader, how many books it has, and which plugin version it runs, with a note when the plugin is older than the one NovelCheck has (the plugin file in the download is the version of your NovelCheck).
- The plugin is parser-checked only; it has not been run on a real device.

## [1.37.0]

### 📊 Reading stats
- **A new Reading stats page** (Profile → Reading sync → **📊 Reading stats**) built from KOReader's reading statistics. **Now:** the book you're reading, today, your last 7 days and your streak. **Timeline:** any day by the hour, with the books you read. **Calendar:** a month and the last year as a heatmap; tap a day for its timeline. **Totals:** 7 days, 30 days, a year or all time, with time, pages, books read, finished, and your most-read books. **Habits:** the hours and days you read most, your longest streak and best day.
- **Your own time zone:** days and hours are worked out on your device, so a late-night read lands on the right day. NovelCheck stores only the reading by the hour (UTC).
- It fills in after KOReader's **Reading statistics → Synchronize now** (it needs the Cloud storage setup). Reading time is time with a page open; long pauses are cut short (10 minutes a page at most).
- Next: stats for people without KOReader (from Hardcover and Goodreads imports), then sharing a profile with the family and shareable cards.

## [1.36.4]

### 🔗 A short catalog address
- **KOReader's OPDS catalog now works at your plain address plus `/opds`** (like `https://novelcheck.example.com/opds`), with your NovelCheck username and your three-word sync code in KOReader's Username and Password boxes. No long private address and no URL shortener. It's the first thing shown in Profile → KOReader setup; the long private address still works, with no login, for those who prefer it.
- The OPDS menu hint now says the catalog is under the 🔍 Search or ⚙ Settings icon.

## [1.36.3]

### 🔑 Sync codes you can type
- **New sync codes are three plain words** (like `blueduckcheese`) instead of 12 random letters, so they're easy to read out and type on an e-reader. Existing codes keep working; press **Make a new sync code** in Profile → KOReader setup to get a word code (KOReader then signs in again with it).
- After 20 wrong sync codes from one address in 10 minutes, KOReader, Cloud storage and the plugin refuse that address for a while, which keeps the shorter codes safe from guessing.
- **Cloud storage stays clean:** typing `dav` as KOReader's start folder now means your own folder instead of making a `dav` folder inside it. **Profile → 🔄 Reading sync → 🧹 Remove empty folders** clears leftovers from earlier attempts (empty folders only; files are never touched).
- **Clearer steps** (Profile → KOReader setup): the five Cloud storage boxes are listed in order, split into "add it once" and "tell Reading statistics to use it".
- The setup steps now explain KOReader's folder picker (hold the bold "Long-press here to choose current folder" line), and say Cloud storage is under the **⚙ Settings** icon in KOReader's top menu.

## [1.36.2]

### 🗂️ WebDAV: see requests that come without a login
- The WebDAV list in Profile → Reading sync now also shows KOReader requests that arrive with no username or sync code (the main admin sees these), so a Kindle that doesn't send its login is no longer invisible.

## [1.36.1]

### 🗂️ Why KOReader's Cloud storage fails
- **Profile → 🔄 Reading sync** now lists the latest Cloud storage (WebDAV) requests: whether the sign-in worked, or why not (no user with that exact name, or the sync code doesn't match). With none listed, nothing has reached NovelCheck, so the address is the problem. KOReader itself only says "Cannot fetch list of folder contents".

## [1.36.0]

### 📚 Libby on Discover
- **Each Discover card shows whether your library lends it:** **📚 Available** (a copy is free now) or **📚 3 holds**; tap it to open the book at your library in Libby. Books your library doesn't lend show nothing. It needs your library set under **Profile → 📚 Libby library**, and answers are kept for a few hours.

### 📱 KOReader plugin: books on the device
- **Report books on this device** (NovelCheck menu in KOReader) sends NovelCheck the list of books on the Kindle: file names, formats and sizes, never folder names. **Profile → 🔄 Reading sync → 📱 Your e-readers** then shows each device, how many books it holds and how many are in your library. Give a device a name with **Device name...**
- This is the second step of the plugin, with the first still untried by me on a real device. Copy the updated `kindle-plugin/novelcheck.koplugin` folder to the Kindle again to get it.

## [1.35.0]

### 📚 Hardcover: send your progress too
- **Profile → 📚 Hardcover → Also send my reading progress to Hardcover** (off until you turn it on). Books you're reading or have finished in Up Next are added to your Hardcover shelves with how far you are, hourly or on **Sync now**. Finishing a book marks it read on Hardcover.
- **Careful by design:** books are matched by ISBN only (a book with no ISBN is skipped), it never lowers progress or a status already further along, nothing is sent twice, and at most 15 books go per run. Anything on Hardcover you've paused or marked "did not finish" is left alone.
- With KOReader's Hardcover plugin you can use either way: it sends progress straight from the Kindle, and NovelCheck sends what it knows from KOReader's sync, so the two agree.

## [1.34.0]

### 📚 Libby: is it at your library?
- **Profile → 📚 Libby library:** type your library's key, the first part of its OverDrive address (`lapl` for lapl.overdrive.com; you can paste the whole address). It's checked and saved.
- **On every book** (Discover, the Library, Up Next), a **📚 Libby** line says whether your library lends the e-book: **available now**, or how many holds, copies and the estimated wait, and whether Kindle is supported. **Borrow in Libby** or **Place a hold in Libby** opens the book at your library, where Libby does the borrowing.
- NovelCheck can't place the hold itself: that needs you signed in to Libby, and there's no way for another app to do that. It uses OverDrive's free public search, so it needs no key, and answers are kept for a few hours.

## [1.33.0]

### 📱 KOReader plugin (step 1)
- **See your Up Next on the e-reader and download from it.** A small KOReader plugin (the folder `kindle-plugin/novelcheck.koplugin` in the NovelCheck project, with install steps in its README) adds **NovelCheck → Up Next** to KOReader's tools menu. It signs in with your username and the sync code from **Profile → KOReader setup**; tap a book to download it. Kids' content rules apply.
- This is the first step. It's new and untried on a real device, so tell me what messages you see. Reporting what's on the device, and later removing finished books, come after it works.

## [1.32.1]

### 📱 Libby and Goodreads imports
- **Libby:** in Libby open **Shelf → Timeline → Actions → Export Timeline → Spreadsheet**, then choose that file under **Import books → Choose a list file**. Each book you've borrowed comes in once, however many times it was borrowed, returned or held. The import page has the steps under **📱 Libby**.
- **Goodreads** already imports from its export file (**Import books**), with your **read** shelf counted as finished; nothing changes there.
- Libby lends books, so these titles aren't files NovelCheck can send to a Kindle: they're for knowing what you've read and checking a book's rating.

## [1.32.0]

### 📚 Hardcover (free)
- **Profile → 📚 Hardcover:** paste your Hardcover API token (hardcover.app → Settings → Hardcover API) and press **Connect**. NovelCheck pulls in the books you're reading and have read, hourly or with **Sync now**.
- **Progress bars and Up Next follow.** A book you're reading on Hardcover shows its page progress in NovelCheck (labeled Hardcover; the newest source wins over KOReader). Books already in your Up Next move to **Reading** when you start them on Hardcover and **Finished** when you mark them read. Nothing is added to Up Next by itself.
- Books are matched by ISBN first, then by title and author. Your token stays on the server, and **Disconnect** forgets it and what was synced.
- It works with the Hardcover plugin for KOReader: it sends your progress from the Kindle to Hardcover, and NovelCheck reads it from there.
- This first version only reads from Hardcover. Sending your NovelCheck progress back to Hardcover comes after it has been tried with a real account.

## [1.31.1]

### 🔧 Fixes
- **KOReader's WebDAV "start folder" works.** KOReader asks for a start folder when you add the NovelCheck folder, and any name gave "can't fetch the folder list". NovelCheck now creates whatever folder KOReader asks for, and the steps say to type `/`.
- **Busy Google Gemini is retried.** When Gemini answers "high demand" (503) or "too many requests", NovelCheck waits a few seconds and tries again, up to three more times, before moving to your fallback model.
- **Is reading sync working?** **Profile → 🔄 Reading sync** shows when KOReader last reported your place (and from which device) and when its statistics last arrived, with a hint when it never has.

## [1.31.0]

### 🔑 Main admin and admin areas
- **One main admin.** The first admin (the earliest account) is now the **main admin**: they reach everything, and they alone make someone an admin and decide what each other admin can reach. **🔑 Make main admin** on another admin's card hands the role over (you keep every area as an ordinary admin). The main admin can't be deleted or demoted until it's handed over.
- **Areas for the other admins.** On an admin's card under **Admin → Users & Rules**, the main admin ticks what they can reach:
  - 🤖 **AI settings**: the AI engine, keys, models, prices and limits, AI machines, custom AI filters.
  - 🧬 **Deep Scans**: starting scans, Review, Deep Scan settings.
  - 📚 **Calibre & cleanup**: the library folder, the Content server, removing books, formats, duplicates, delete requests.
  - ✉️ **Email & Discover**: Send-to-Kindle email, the Discover lists and the NYT key.
  - 🖥️ **System**: features, remote access, safe mode, backups, updates, system checks, Usage.
  - 👥 **Users**: the other parents' accounts. Every admin can always manage kids.
- What an admin can't reach is hidden in the app and refused by NovelCheck, even through a link. Their Admin page lists their access at the top.
- **After this update** every existing admin keeps everything they had, so nothing changes until the main admin unticks areas. **New admins** start with Deep Scans only.
- Editors work as before.

## [1.30.1]

### 🔧 Fixes
- **No spoilers in "What the AI found".** A Deep Scanned book's window now lists only where each part is and its level. What happens there shows when you open **📖 Read this part in the book**, at the top of the highlighted part. The notes a review decision adds to a book no longer describe the scenes, and Review keeps the description folded under **What happens (spoilers)**.
- **Faith shelves leave out the occult.** Advent, Lent & Easter, and Saints & feast days never show books flagged Dark Occult / Demonic, whether the shelf is found by words or built with the AI. A parent's ✓ OK still counts.
- **Google Gemini works again.** The Gemini preset now uses `gemini-3.5-flash-lite` (with `gemini-3.1-flash-lite` as the fallback): Google limits the older 2.5 models to accounts that already used them. Gemini's thinking is kept low with room for the answer, and its errors now show Google's own message instead of a cut-off code. Already set up with Gemini? Change **Primary (small) model** to `gemini-3.5-flash-lite` under Admin → AI & Scans.
- **Seasonal chips and card rows aren't cut off.** The outline of an in-season chip, and of the cards in Discover and Suggested Reads, was clipped at the top.

## [1.30.0]

### 👥 Who's reading? (family devices)
- **A shared tablet or computer can open on everyone's profiles.** A parent turns it on in **Profile → Family device & PIN → Use this device for the whole family**. The device then opens on **Who's reading?**: kids tap their name, and anyone with a PIN types it on a big keypad.
- **Parents stay protected.** A parent profile asks for its **4-digit PIN**, or the password if no PIN is set, so a kid can't tap into a parent's account. After 5 wrong tries, that profile is locked on that device for a while.
- **Switch profile** replaces **Sign out** on a family device, in the top bar and in More on phones.
- **PINs:** set or change your own in Profile (your password confirms it). Parents can set a kid's PIN with **🔢 PIN** on their card under Admin → Users, where family devices are also listed and can be removed.
- Signing in on your own devices doesn't change.

### 🔧 Fixes
- **Start page works.** NovelCheck now opens on the page you picked in **Profile → Start page** when you open it from the home screen, sign in, or switch profile. Before, the home-screen app always opened on the Library, and signing in kept the page from before. A reload still stays where you are.

## [1.29.0]

### 📖 Reading progress
- **How far you are, everywhere.** Books you read in KOReader (or an app that uses KOReader's progress sync, like Readest) show a progress bar in **Up Next** with the device and when you last read. A queued book you've started elsewhere shows its %, and a book's window says **Your progress**. With several devices, the newest place wins.
- **📱 On your KOReader.** Point KOReader's **Reading statistics → Cloud sync** at NovelCheck (**KOReader setup** shows the steps) and Up Next lists every book opened on your KOReader devices, with how far you are, how long you've read and when. That includes books that didn't come from NovelCheck, and it combines all your KOReader devices. Books already in Up Next move to Reading when opened and to Finished at the end. Other books aren't added, since the statistics go back years.
- **Parents see a kid's reading** on their card under **Admin → Users**: what they're reading now, how far along, and what they opened lately in KOReader.

### 🔧 Fixes
- **The Deep Scan "small model" warning checks the right machine.** With a Deep Scan machine set up under **AI machines**, it now looks at that machine's model instead of the main AI's. **Keep using it** hides the warning for a smaller model you chose on purpose.

## [1.28.1]

### 🔧 Fixes
- **Up Next no longer tries to email a book there's no file for.** A book with nothing Amazon takes, like a paper book, one from an imported Kindle or Goodreads list, or one only in AZW3/MOBI, says so on its row. **▶** then just marks it as reading instead of failing with an error. For AZW3/MOBI-only books, an admin can make an EPUB with **Formats… → Convert to EPUB**.

## [1.28.0]

### 🗂 Series
- **A Series page** (Collections → **Series**): every series in your libraries, with how many you've read. Open one to see it in order: the numbers you don't have, what you've read or are reading, and **Next up** with **＋ Up Next**.
- A book's series name opens its series page.

### 📦 Box sets
- **Books that hold several books** ("Books 1–3", "Trilogy", "Box Set") are found by their titles, and parents see a 📦 banner in Admin. A parent checks what's inside (**✨ Ask the AI** can list it), then splits it, or marks it **Not a box set**.
- **Split books are real books:** each gets its own card and rating, counts as yours, and opens and downloads the box set's file. **Undo the split** puts it back.
- The box set's file can't be removed or renamed from a split book, only from the box set.

### 🧬 Deep Scan details on every scanned book
- In the window of any Deep Scanned book, parents (admins and editors) get **What the AI found**: each part it noted, what happens in the scene, and **📖 Read this part in the book**, the same reader as in Review. Until now these were only there for scans waiting for review. Kids see the short list of parts as before.

### 🎯 Shelves that stay on theme
- **Stricter AI picks.** When the AI fills a collection or builds a seasonal shelf, it keeps only books that are clearly *about* the theme (their subject, setting or main characters), not ones that just share a mood or a lesson. It no longer tries to fill the list, and it checks each pick a second time with a differently worded question. A theme "for families" or "for ages 8–10" no longer pulls in every children's book.
- **Seasonal shelves match whole words.** "Saint" finds saints but not "saintly" or "Saint Louis", "Lent" no longer catches "lent him a book", and vague phrases like "lives of the" are gone.
- **✕ Not for this shelf.** On a collection or seasonal shelf, **☑ Select** books and take them off it. They stay in your libraries, and neither the shelf's words nor the AI (**Build again**, **Find more**) bring them back.
- **🔍 Check these books** (on a shelf's banner, parents) has the AI read every book on the shelf and list the ones that don't fit, with a reason. Untick any to keep, and the rest come off the shelf.

### 🔗 Same book, other titles
- **TV and movie tie-ins** ("Dune (Movie Tie-In)", "Title: Now a Major Motion Picture") are recognized as the same book as the original.
- **Same book as…** (on a book, for parents) joins two cards for one book, like a reissue under another name. The better-rated one stays, and its libraries, Up Next places, notes and wishes come together. Later imports under the other title find it.

### 📖 Reading sync
- **KOReader progress sync.** Point KOReader's **Progress sync** at NovelCheck (Profile → **KOReader setup** shows how, with your sync code). Opening a book marks it **▶ Reading** in Up Next, and reaching the end marks it **✓ Finished**; KOReader also keeps your place between devices. Parents can set it up for a kid's device from **Users**.
- **Read shelves on import.** Importing a Goodreads or StoryGraph export marks the books on your **read** shelf as finished.
- Stock Kindles can't report what's been read; mark those books finished in Up Next.

### 🗂️ Formats in Calibre (Admin → Delivery & Services → Calibre Library → Formats…)
- **Keep only the formats you want.** Tick the formats to keep (say EPUB): every book that has one loses its other formats, into Calibre's recycle bin. Books without a kept format are left alone, and NovelCheck checks with Calibre first. You see the list and the space saved before anything happens.
- **Undo for 7 days** puts a whole cleanup, or one file, back from Calibre's recycle bin.
- **Convert to EPUB.** Books with no EPUB but a MOBI, AZW3 or similar file can be converted by Calibre, one at a time, with its own conversion settings. The original file stays.
- Uses the Content server connection from **One-click removal**, with a Calibre user allowed to make changes.

### 🔧 Fixes
- On phones, the safe mode banner and a shelf's banner put their buttons under the text instead of squeezing the text into a narrow column.

## [1.27.0]

### 🧬 Deep Scan: fewer false alarms, and you decide
- **A stricter second look.** For a part rated Level 3 or more, the AI now says how far the scene goes (kissing, making out, sexual touching, implied or cut away, briefly described, described in detail). A part counts as Level 4 or more only when the sex act is described in detail, the AI names the sentences that describe it and they really are in the book, and a second, differently worded question agrees. Scenes that fade out or cut to the next morning stay at Level 3.
- **One passage, your call.** A raise to Level 4 or more that rests on a single passage always waits for your review, however small the raise.
- **Read it yourself.** In **Review**, each flagged part has a short description of the scene and **📖 Read this part in the book**: the part the AI read, highlighted, with the text before and after it and **◀ Earlier / Later ▶**.
- **Any level.** Besides Accept and Keep, **Or set Level…** picks the level in between when that's the right one.
- **A note for parents.** Every decision adds a parents-only note to the book: the scenes the scan found, where, and the level chosen.
- Scans made before 1.27 aren't changed: decide them in Review with the reader and **Or set Level…**, or scan the book again.

### 🧭 Discover
- **Opens at once** with the lists you saw last time, then refreshes them. Each row keeps where you'd scrolled to.
- **Much faster to load:** seasonal shelves (like Fall and Halloween) no longer search every book description on each visit.
- **📰 NYT list** tags books from The New York Times Best Sellers lists (with how many weeks they've been on), and each such row credits The New York Times.
- The New York Times lists are fetched **once a week** (they change weekly). **Refresh lists now** still fetches them straight away.

### 🎨 Appearance (Profile → Appearance)
- **A light theme.** NovelCheck follows each device's light or dark setting, or you can pick Dark or Light.
- **OpenDyslexic,** a font some people with dyslexia find easier to read.
- **Reduce motion** turns off animations and smooth scrolling (it also follows the device's setting).
- These are per person and follow you to every device.

### 🔎 Search
- The Library remembers your **last 10 searches**; they appear under the search box (with **Clear**) on every device you use.
- **Sort: Title** ignores "The", "A" and "An"; **Sort: Author** goes by last name; there's also **Sort: Fewest peppers**.

### 🛟 Safe mode and commands
- **Safe mode** starts NovelCheck without its background work (Calibre sync, rating, Deep Scans). It turns on by itself if NovelCheck stops unexpectedly 3 times within a few minutes of starting; admins see a banner with **Leave safe mode**.
- **Commands** for the TrueNAS app's Shell: `novelcheck reset-password <user>` (for a locked-out admin), `novelcheck backup`, `novelcheck check`, `novelcheck users` and `novelcheck safe-mode on|off`. `novelcheck help` lists them.

### 📱 Phones
- **Admin:** one button at the top names the section you're in and opens all sections as a sheet from the bottom, like More.
- **Up Next:** adding a suggestion to Up Next no longer moves the suggestions on iPhone.

## [1.26.1]

### 🖥️ TrueNAS: surer starts
- **NovelCheck no longer has to download itself at every boot.** The install guide now keeps TrueNAS's default pull policy (*only pull the image if it isn't on the NAS yet*), so NovelCheck starts from its own copy even before the internet is up. With *Always*, TrueNAS downloaded it on every start and could fail with "Timed out waiting for response". Installed with *Always*? The end of **Updating NovelCheck** in the TrueNAS guide shows how to switch; updates then come through TrueNAS's **Update** button.

### 🔧 Fixes
- **Admin → Deep Scan → Settings works again.** Until readers were picked for automatic Deep Scans, the section showed no accounts, and **Estimate cost…** and **Save** did nothing.
- **The version under the menu is right after an update.** `:latest` is now published only by a release, so the app no longer shows a label like "1.24.0-1-…" after updating.

## [1.26.0]

### 🎉 Events
- **Whole lists from "Load more books" pages.** Give NovelCheck an event page's address and it presses **Load more books** for you (and follows next pages), so it finds every book, not just the first ten. It also picks up each book's author, and fills in the event's name and day from the page.
- **An event opens in the Library,** with every filter and sort: peppers (including ⚠ Needs review), content, age group, genre, author, hidden content and **Hide books we have**; sort by the event's order, title, author or **Fewest peppers**. **Claim on Amazon ↗**, **✓ I claimed it** and **⭐ Wishlist** are on each card.
- **Events end on their own.** Give an event an end day and time (filled in when the page says its day). When it ends, it moves to the **🗄 Archive** with its books and ratings. Without an end, an event is archived after 30 days unless pinned.
- **Archive instead of delete.** Parents can **🗄 Archive** or **↩ Restore** an event and **✏️ Edit** its name or end. **🗑 Delete** is still there.

### 🤖 AI machines (Admin → AI & Scans → AI machines)
- **Deep Scans on a bigger machine.** Point Deep Scans at a separate, stronger AI machine (the Ollama easy setup works for it too). Ratings stay on the main AI. While that machine is off or asleep, Deep Scans wait (**🧬⏸** in the top bar, and a note on the Deep Scan page) and carry on by themselves when it answers.
- **Model updates.** Once a day NovelCheck checks whether a newer version of the Ollama models you use is out and sends a 🔔 notice; **Update** downloads it with one click.
- **Speed test.** Times each model on a made-up sample book: one rating and one Deep Scan part. Deep Scan estimates then say how long a book takes.
- **Electricity cost.** Enter your price per kWh and each machine's extra watts while it works. **Usage** then counts what your own AI machines cost in electricity, and Deep Scan estimates include it.
- **⚠ Needs review.** The AI now says how sure it is of each rating. Ratings it wasn't sure of (often books with little description) get a **⚠ Not sure** chip and show under **⚠ Needs review** in the Library's pepper filter; the book's window offers **Re-rate with the big model** when a Deep Scan machine is set up.
- **Thinking models.** Reasoning models (such as DeepSeek-R1, QwQ or Qwen3) get room to think before they answer, and their thinking is left out of the rating.

### 🔧 Fixes
- The 1.25 notes under **What's new** show properly again.
- A claimed book's window no longer shows an internal code under its library.

## [1.25.0]

### 🎉 Stuff Your Kindle events
- **Free-book days, sorted for your family.** Under **Discover → 🎉 Events**, a parent presses **＋ New event**, then pastes the event's list (copy it from the event's page or email: the Amazon links come along) or types the page's address.
- **Every book is rated.** Books NovelCheck doesn't know yet are rated first in line, within your hourly AI limit, and the page fills in as the ratings arrive.
- **See what fits:** each book shows its peppers and content icons, and whether it's already yours, on a wishlist or in Up Next. Tick **Hide Level 3+**, **Hide books we have** or **Rated only**, or sort **Mildest first**.
- **Claim on Amazon ↗** opens the book's Amazon page. Then **✓ I claimed it** puts it in the library you pick (for example "Kindle (Amazon)"), so it counts as yours everywhere.
- **The whole family can look.** Kids see the rated books their rules allow, without the Amazon buttons. Events clean themselves up after 30 days unless a parent pins them; **🗑** discards one sooner.

## [1.24.0]

### 🧭 A shorter menu
- **Check a book · Library · Discover · Up Next · Collections · Admin · Profile.** That's all that's left at the top (on phones: Check, Library, Discover, Up Next and More).
- **Deep Scan, Usage and System checks are tabs in Admin**, next to AI & Scans, Users & Rules and the rest.
- **⭐ Wishlist is inside Discover:** a Discover | Wishlist switch at the top of both.
- **Import books and 📕 Paper books are under Profile → ➕ Add books.**
- **Pick your start page** under **Profile → Start page**. By default parents open on Check a book and kids on the Library.

### 📕 Asks instead of guessing
- **Typing a title in Paper books or Check a book no longer adds the wrong book.** If Open Library's best match doesn't contain every word you typed (or several books do), NovelCheck shows the likely books with author and year to pick from, or **None of these: use it as typed**. Barcodes are still added straight away.

### 🧬 Deep Scan
- **The Running list is in the order the scans run:** the one reading now, then Next, #2 in line and so on, then requests. **Drag ⠿** to change the order.
- **Scans stop failing on local AIs.** Parts now fit your AI's context window (4,096 tokens unless you raised it: **Admin → AI & Scans → Rate Caps → Context size of your local AI**). A part the AI still calls too long is split in two, and a fumbled answer is tried again up to 3 times before the scan fails. This fixes "exceeds the available context size", "level must be 0-5" and "token repeat limit reached".
- **A server that's down or stuck no longer costs one wait per model:** NovelCheck moves straight to your backup AI (for ratings too).

### 👧 Kids and AI
- **Parents decide whether each kid gets AI features:** Suggested Reads picked by the AI, and asking for a Deep Scan. Tick **🤖 Allow AI features** on a kid's card under **Users & Rules**. It's off until you do; kids still get suggestions from free matching.

### 🔧 Fixes
- **Suggested Reads keeps its place:** 👍, 👎 or ＋ on a book no longer jumps the row back to the start.

## [1.23.0]

### 📚 Collections
- **Shelves across your libraries.** The new **📚 Collections** page (under **More** on a phone) lists them with covers; tap one to browse it in the Library, where search and filters still work.
- **Make one yourself:** **＋ New collection**, then add books from each book's window (**＋ Add to a collection**). Parents can take a book out with ✕ there too.
- **Let the AI fill one:** press **✨ Describe one for the AI** and write what you want ("dragon adventures for ages 8-10, not too scary"). The AI looks through your libraries and suggests books with a reason each; untick any you don't want and save. **✨ Find more** later adds new books that fit, again for you to check first.
- **Ideas from the AI:** about once a week the AI proposes up to three themed collections from your library, already filled. **Keep** or **Drop** them on the Collections page. On by default with a local AI, off with a paid one (**Admin → AI & Scans → Automatic rating**); it keeps to the automatic-rating hours.

### 👧 Kids limited to collections
- Under **Users & Rules**, a kid's account can be set to **Only books in chosen collections**. They then see no other books anywhere (Library, search, Discover, suggestions), not even ones you marked OK, and their content rules still apply on top.

### 🗓️ Seasonal shelves
- 🎃 Halloween, 🍂 Fall & Thanksgiving, 🕯️ Advent, 🎄 Christmas, ❄️ Winter, 💝 Valentine's & friendship (Level 2 at most), ✝️ Lent & Easter, 🌷 Spring, ☀️ Summer, 🎒 Back to school and 👼 Saints & feast days. The church seasons follow Easter and Advent each year.
- **The ones in season come first** in a row of chips at the top of the Library and Discover, and Discover opens with your libraries' books for the season. **🗓️ More seasons** shows the rest.
- Books are found by words in titles, Calibre tags and descriptions. For a better pick, parents can press **✨ Build with AI** on a season: the AI builds that season's collection, and the shelf uses it from then on.

### 🔧 Fixes
- **Phones:** with the activity pill showing, the header wrapped onto two lines. **Sign out** is now at the bottom of **More** on phones (the More tab is always there).

## [1.22.0]

### ⚡ Books rate themselves
- **No more "Analyze next batch" to get new books rated.** When the AI has nothing to do, NovelCheck rates the books waiting for a rating by itself: books in someone's Up Next or wishlist first, then the newest. It starts right after a Calibre sync or an import and keeps to the hourly token limit.
- **On by default with a local AI** (Ollama and the like, which cost nothing). With a paid AI it stays off until you turn it on, so there's no surprise bill. **Admin → AI & Scans → Automatic rating** has the switch and says how many books are waiting.
- **Quiet hours:** rate automatically only between set hours (say 11 pm to 7 am) so the GPU is free the rest of the day. Check a book, paper books and "Analyze next batch" still work any time.
- After a Calibre sync or an import, the notice says the new books will be rated automatically, with a **Watch progress** link.

### 👀 See what's happening
- **Clear status on every unrated book:** ⏳ Waiting, ⏳ In line, ⚡ Rating now… and ⚠ Rating failed, instead of a grey "Pending Analysis".
- **The Library updates as you watch:** the book being rated shows ⚡, a Deep Scan shows 🧬 Part 4 of 12, and each rating appears on its card as soon as it's saved, without reloading the page.
- **Activity in the header** for parents: ⚡ 38 while books are being rated (38 waiting), ⏸ while paused for the hourly limit, 🌙 while waiting for the rating hours, and 🧬 4/12 during a Deep Scan. Tap it for the waiting books or the Deep Scan page.
- **＋ turns into ✓** on Library cards once a book is in your Up Next (and books already there show ✓), with a **View Up Next** link.

## [1.21.0]

### 📕 Paper books
- **Your printed books, rated like the rest.** Open **📚 Paper books** (under **More** on a phone), pick or create a library such as "Living room shelf" or "Kids' room", and press **▦ Scan barcodes**. Hold up one book after another: each is looked up, added and rated.
- **No barcode?** Type the title or ISBN, or take a photo of the cover.
- **Ratings come in as you scan.** New books are rated one after another, first in line and within your AI's hourly limit, and the list shows each rating as it arrives. Tap a book for its details, or ✕ to take it back off.
- **Paper books are everywhere your e-books are:** the Library (with a 📕 Paper chip, and **📕 Paper books** under Format), Discover, Up Next and Suggested Reads, and kids see them under their usual rules. **▶ Start Reading** on a paper book just marks it as reading.
- Parents add paper books; the libraries belong to the family, so every parent can add to them. Import books links here too.

## [1.20.1]

### 📱 Phones
- **Up Next is readable.** Titles get two full lines, and ▶ (start reading) and ✕ are small buttons beside them. Tap a title or cover to open the book.
- **No more sideways scrolling, and the tabs stay put.** A drop-down with a long library name (such as "From" under Suggested Reads) could make a page wider than the screen. The bottom tabs then slid with it and **More** ended up off the edge. Nothing can widen a page any more.
- **More opens from the bottom**, just above the tabs, with big buttons for every other page. Swipe it down or tap outside to close it. Admins now find **🧬 Deep Scan** there (and in the top menu on computers).
- **Pop-up windows open as sheets from the bottom.** In the book window, ✕ stays at the top, and the main buttons (＋ Up Next, ⭐ Wishlist, Edit rating) stay at the bottom while you scroll.
- **Updates show up right away.** Phones could keep parts of the old version for an hour or two after an update, and a mix of old and new files could break pages. Each version's files now have their own addresses, so a phone always runs one version. An app left open picks up a new version when you come back to it.

### 🧬 Deep Scan page
- **Four sections:** Review, Running, Results and Settings. The page opens on whatever needs you.
- **Accept or Keep a raise without losing your place.** The card goes away, the count drops, and the page stays where it was. **Accept all** joins **Keep all old ratings**.
- **Tap a title or cover** to open the book. What the AI found is folded under each card, so a long list is quick to scan.
- **Running scans** show a progress bar.

### 🔔 Notifications
- **Deep Scans waiting for review are one card**: "14 Deep Scans wait for your review", with a **Review** button and the list of books.
- **Each notice has a clear button** (Review, Open System…) and a ✕ to dismiss it. Copy and Diagnose appear only on errors.
- A notice is marked read when you follow its button, and a Deep Scan's notice clears when you decide it.

### 🧭 Discover
- **Discover shows books you don't have.** Books already in your libraries are left out of the lists (Open Library's lists often repeat them), and the rows are refilled from longer lists. Tick **Also show books we already have** to see them again.
- **Books you don't own open properly.** The book window treated a best-seller from Discover as one of your books. It now offers ⭐ **Add to Wishlist** and **Up Next (to get)**, and says it's not in your libraries yet.

## [1.20.0]

### 🧭 Discover
- **A new tab of books to swipe through:** 🔥 popular now, 🆕 new on the best-seller lists, 📰 popular nonfiction, 🧑‍🎓 top teen books, 🧒 top kids' books, 🏛️ all-time classics, 📚 new in your libraries and ❤️ popular in the family.
- **Your rules apply.** Every book shows its peppers and content icons, everyone's content rules apply, and kids never see a book the AI hasn't rated yet. Younger kids see the kids' rows first; teens see the teen row first.
- **Real best-seller lists** with a free New York Times key (**Admin → Delivery & Services → Discover**; the steps are in the guide linked there). Without a key, those rows come from Open Library.
- **Rated in the background.** NovelCheck rates about 30 list books a day (you can change the number), best-ranked first, within your AI's hourly limit.
- **Get it:** ⭐ **Wishlist** for books you don't have, ＋ **Up Next** for ones you do, and links to Amazon and Open Library. Kids don't get the Amazon link.
- Discover books you don't own stay out of your Library. On phones, Discover is a tab at the bottom and Wishlist moved into **More**.

### 📖 Real blurbs on book cards
- **Cards now say what the book is about**: one or two spoiler-free sentences the AI writes when it rates a book (who, where, and what's at stake). There's no praise, "#1 bestseller" or rating talk.
- Until the AI has written one, cards show the start of the book's own description, with review quotes and best-seller lines taken out.
- The AI's rating note moved to the book window, under the blurb, labelled **Rating note**.
- **Your books get their blurbs with the next re-rate** (the Admin banner offers it). Books you rated yourself and Deep Scanned books keep their rating: the AI only adds the blurb and content details. If you haven't run the 1.19 re-rate yet, one re-rate now does both.

### 🔧 Fixes
- **Phones: no more sideways scrolling.** A long title or series name could stretch the Library's cards, and with them the whole page, wider than the screen.
- **Deep Scan review works.** Scans waiting for you could drop off the Deep Scan page once enough newer scans had finished, so **Review** showed nothing to review. They're now always listed first, at the top of the page, and **Keep all old ratings** turns them all down at once.
- **Tidier titles:** `**` marks and a trailing ": A Novel" are removed from titles, including ones already in NovelCheck (Calibre keeps its own title; the book window offers to tidy it there).

## [1.19.0]

### 🧩 Content details: language, violence, gore, substances and more
- **45 things the AI now looks for**, in five groups: 🗣️ Language, ⚔️ Violence, 🩸 Gore, 🍺 Substance Use and 🧩 Other Content (such as self-harm, bullying, grief or religious themes). Peppers still rate romance on their own.
- **See it at a glance.** Book cards show an icon for each group a book contains. The book window lists the items and says where they came from: the description, a Deep Scan (the whole book) or a parent.
- **How much, from a Deep Scan.** Books read in full also show how much of each group there is: A little, Some or A lot.
- **Hide exactly what you want.** In the Library, **Hide content** hides a whole group, or open a group and tick single items (just Torture and Gun violence, say).
- **Kids' rules.** Each account under **Users & Rules** has the same **Hide content** choices, and kids' rules can now use your custom filters too. **Strict Family** ticks a starter set: the F-word, sexual language, slurs, torture, sexual violence, violence against children, dismemberment, mutilation, graphic deaths, drug dealing, overdose and self-harm. **Young Reader** adds more, such as swearing, murder and drugs. New kid accounts start with the set for their age group. Existing kids' accounts don't change until you tick items or press a preset. You can change any of it.
- **Fix what the AI got wrong.** **Edit rating** has the items too. Books a parent marked **OK** still show for everyone.
- **LGBTQ+ moved** into 🧩 Other Content. Books and kids' rules keep what they had.

### ⏳ Checking the books you already have
- The **re-rate** banner offers to check your books for the new details. It runs in the background; on a local AI a big library takes a few hours. Books the AI rated are re-rated as usual. Books you rated yourself and Deep Scanned books keep their rating: the AI only adds their content details.
- Until a book is checked, kids who have content rules and **Hide books not yet analyzed** won't see it, unless a parent set its age group or marked it OK. Other kids see it under their existing rules.
- Deep Scans read a little more per part, so their cost estimate is a bit higher.

## [1.18.2]

### 📱 Easier on phones
- **Tabs at the bottom.** On a phone, the menu that used to wrap across the top is now a tab bar within reach of your thumb: the first four of Check, Library, Up Next, Wishlist and Profile, with **More** for everything else.
- **No more zooming in.** Tapping a search box or form field on an iPhone no longer zooms the page.
- **Admin pages fit.** On a phone, the Admin tabs sit in a 2×2 grid and the number tiles two to a row.

### 🌶️ Pepper levels in the family's own words
- **Level 2 is now "Romantic"**: developed romance and passionate kissing, with no sexual desire and no hint of sex. It used to allow intimacy off the page.
- **Level 3 is now "Steamy Closed-Door"**: strong sexual attraction and desire, heavy making out, and any sex happens off the page or fades to black.
- **Level 5 is now "Very Explicit / Erotica-Level".**
- **Everyday moments don't count.** Sharing a glass of wine or waiting for a taxi is not romance.
- **Vampires and werewolves are ordinary fantasy.** They don't make a book steamier or "dark occult" on their own. For reference, the AI is told *Twilight* is about Level 2 and *Dead Until Dark* is Level 4.
- **Deep Scan uses the new Level 3.** Desire, heavy making out or sex off the page count as Level 3; only sex on the page counts as 4 or more.
- **Re-rating.** Because the levels changed, books the AI rated before this update are offered for re-rating in the Admin banner. Books you rated yourself and Deep Scan results are never changed. Once re-rated, books where sex happens off the page move from Level 2 to 3, so kids limited to Level 2 (Strict Family) no longer see them.

### 🔧 Fixes
- **AI models that "think" first** (such as DeepSeek-R1) no longer break ratings. If one runs out of room while thinking, the error now says so.
- **Lists copied from AI chats**: bold marks (`**`) are removed from titles and authors when you import a list.
- **The guide** now tells editor accounts that their admin tab is called **Manage**.

## [1.18.1]

### 🧬 Deep Scan: stricter and fairer
- **Peppers are only about romance.** A small AI model was rating exciting chapters as "explicit": a battle in *Harry Potter and the Chamber of Secrets*, an emergency on the spaceship in *2001*. One such chapter could make the whole book Level 4. Now fights, deaths, monsters, danger, tension and magic never count.
- **It has to name what it saw.** For each part of the book, the AI first says what romance or sexual content is on the page, or "none", which means Level 0. A part rated 3 or higher gets a second, stricter check before it counts. Repeating the pepper scale's own wording doesn't count as evidence.
- **One part can't decide a book.** In longer books a content tag such as Heavy Innuendo must show up in two parts. Innuendo and nudity only count where sexual content was confirmed, and Level 5 needs several explicit parts.
- **Big jumps wait for you.** A Deep Scan that would raise a book by 2 or more levels waits on the Deep Scan page, showing what it found, until you **Accept** it or **Keep** the old rating.
- **Old results were put back.** Books Deep Scanned before this update go back to their rating from the description (you'll see a notice). You can Deep Scan them again any time.
- **Model size advice.** Deep Scan warns when it uses a small model (under 7B). If a bigger one is already on your Ollama, **Use it for Deep Scan** switches in one tap; other ratings keep their model.

## [1.18.0]

### 💡 Suggested Reads
- **Suggestions under Up Next**: a row of books from your library picked for each person, each with a short reason, such as "Next in Discworld after Mort", "More by Terry Pratchett" or "Like The Hobbit". They're based on what that person has in Up Next, is reading and has finished. Kids only ever see books their rules allow.
- **👍 / 👎**: 👍 means "more like this" and shows **＋ Up Next** (or **⭐ Wishlist**). 👎 hides the book at once and asks an optional **Why not?**: *Not my kind of story*, *Not this author*, *Not this series*, *Too spicy* or *Already read it*. Each answer teaches the suggestions something different: a 👎 for "Too spicy" stops books at that pepper level or higher, "Not this series" skips the rest of the series, and "Already read it" moves the series on to the next book. **Start over** clears your 👍/👎.
- **The admin decides how they're picked** (Admin → AI & Scans → Suggested Reads): **Free matching only** (no AI, no cost), **AI picks from your library** (your AI picks the best 10 and writes the reasons, at most once a day per person and within the hourly token cap), or **AI picks + books you don't own** (a few books to add to the wishlist, where NovelCheck rates them). Kids' accounts never get books you don't own. **Admin → System & Toggles → Features** can switch Suggested Reads off.

### 🎯 Your reading taste
- **Tell NovelCheck what you like (optional)**: open **🎯 Your taste** from Suggested Reads or your Profile to see 20 books from the library. Mark the ones you know: **📖 Want to read**, **❤️ Read & liked**, **👎 Read, didn't like** or **🙅 Not for me**, and skip the rest. **Show 20 more** brings a new batch any time, and **Your answers** lets you change or remove any answer (your 👍/👎 on suggestions are listed there too).
- Answers go straight into your suggestions: books you've read and liked count as reading history, so a series moves on to the next book. The list mixes different authors and pepper levels, starting with the first books of a series. Marking books uses no AI. Admins can switch it off under **Features**.

### 🖼️ Book covers
- **Covers everywhere**: the Library, Up Next, Suggested Reads, the taste list, the wishlist, Check a book and each book's window now show the book's cover. It comes straight from your Calibre library, where Calibre keeps one next to each book's files. NovelCheck makes a small copy once and never changes the library. Books you only looked up get their cover from Open Library by ISBN, and books with no cover get a tidy stand-in with the title.
- **🖼️ Wrong cover?**: anyone can report a wrong cover from a book's window, with an optional note. Admins see **🖼️ … covers were reported as wrong** on the Admin page, with the cover, the note and an **Open in Calibre-Web** link. Change the cover in Calibre and press **Fixed**; NovelCheck shows the new cover by itself.

### 🔎 Browse the library
- **New filters**: **genre** (Fantasy, Romance, Mystery & Thriller, Science Fiction, Horror, Historical Fiction, Young Adult, Children's, Biography & Memoir, History, Self-help & more, each with its number of books), **Fiction / Nonfiction**, **author** and **series** (both suggest names as you type). Search now also looks at series names and Calibre tags.
- Genres come from the **tags in Calibre**, matched on whole words (so "Science Fiction" isn't "Science"), and a "Fiction" or "Nonfiction" tag decides which it is. Books with no tags can get theirs from the AI: the Admin page shows how many are missing and a **Fill in with AI** button with the estimated cost. It sorts about 25 books per AI call, stays within the hourly token cap and can be stopped. Tags you add in Calibre always win.
- In a book's window, the **author**, **series** and **genre** are links that open the library filtered to them.
- **Suggested Reads: From** lets each person get suggestions from every library or just one, for example a Kindle they imported. It's remembered on their device.

### 🐞 Report a problem or idea
- **No GitHub account needed**: anyone can press **🐞 Report a problem or idea** at the bottom of any page. It goes to the admins (in the 🔔 and on the Admin page), with the page they were on and their device type. From there an admin can **🩺 Diagnose** it, which checks the setup and can pass a real NovelCheck bug on to the developer, or copy it or mark it done.

### 📚 Your own libraries
- **Libraries have owners**: a library you import (a Kindle, a folder or a list) is yours. Tick **🔒 Private** when importing, or later under **📚 Your libraries** on the Import page, and only you and the admins see its books. **Shared** libraries are seen by everyone, and kids still only see what their rules allow.
- **Remove books from your own library straight away**: in a book's window, **Remove from this library** takes it off that library's list at once, with no delete request (nothing is deleted from your device or from Calibre). You can also rename or delete your own libraries. Admins can change who owns a library; libraries imported before this update belong to the whole family.

### ☑ Select several books
- **☑ Select** in the Library lets you pick many books at once: tap them (on a computer you can also drag across them, or Shift-click to pick everything in between), or **Select all shown**. Then:
  - **＋ Up Next** adds them all.
  - **🧬 Deep Scan** starts scans for an admin, or asks an admin for everyone else. Books without an EPUB file are skipped.
  - **🗑 Delete** takes books in your own libraries out straight away, and sends a delete request to the admins for the rest.

### Changed
- The **How-to guide** covers the new features, and every "Admin → …" instruction in the app and the guides now names the Admin tab it's on (for example **Admin → Delivery & Services → Calibre Library**).
- The app says **library** everywhere it used to say "catalog" (the KOReader catalog keeps its name).

### Fixed
- Tidying titles in Calibre without the Content server set up pointed to the wrong Admin page.

## [1.17.0]

### 🧬 Deep Scan
- **The AI can now read the whole book**: open a book and choose **🧬 Deep Scan this book** (admins), or **Request a Deep Scan** (everyone else; an admin approves it). NovelCheck reads the book's EPUB from Calibre part by part, and the rating comes from the text itself instead of the description. The book window lists what was found where (for example "Chapter 12 · Level 4: an explicit scene"), and the book gets a **🧬 Deep Scan** tag.
- **Cost first**: every Deep Scan shows its size and estimated cost before it starts (a typical novel is around 150,000 tokens; with gpt-4o-mini that's about 2–5 cents).
- **Admin → 🧬 Deep Scan**: scan the next 10, 20 or 30 books waiting in Up Next lists (with the estimate shown first), pick up to 3 readers whose Up Next is always scanned automatically, approve requests, follow progress, and see every rating the full text changed.
- **When the text is spicier than the blurb**: the rating is updated, the 🔔 bell (and phones) get a warning, and the book shows **⚠️ Rating changed via Deep Scan: Level 2 → Level 4** in Up Next and in its window.
- **Library → 🧬 Deep Scanned only** shows just the fully read books. Deep Scan ratings are never overwritten by re-rates of the description.
- Optional **Deep Scan model** under Admin → LLM Analysis Engine (for example a cheaper model with a big context window); otherwise your usual AI does the reading.

### 📷 Barcode scanning & ⭐ Wishlist
- **Scan the barcode live**: Check a book has a new **▦ Scan the barcode** button. Point the phone at the barcode on the back of a book and it's recognized in a moment, on iPhone and Android alike (it needs NovelCheck's https:// address). If the camera can't be used, you're told why and can take a photo instead.
- **Already have it?** Books in your library show **✓ In your library** with **＋ Add to Up Next**. Books you don't have show **⭐ Add to Wishlist** or **＋ Up Next (to get)**, and never offer deleting or removing.
- **⭐ Wishlist** (new tab): everyone can wish for books, with a note. Parents see the whole family's wishes, **Approve & Track** the ones they'll buy, decline others, or mark them as got; a wish is marked as got by itself once the book shows up in your library.
- **Up Next** shows **📦 Pending acquisition** for books you don't have yet (instead of Start Reading).

### 📬 Delivery
- **KOReader catalog**: every reader now has a private catalog address for KOReader (**Profile → KOReader setup**, with a QR code and step-by-step instructions). Add it in KOReader under 🔍 → **OPDS catalog** and the Up Next books appear there, ready to download. Parents can open it for a kid's device from the kid's account card, and a kid's catalog follows the kid's content rules. **New address** retires the old one.
- **Send-to-Kindle check**: pressing **▶ Start Reading** with Send-to-Kindle first shows where the book goes and which address it comes from, with the steps to add that address to Amazon's approved list. **Profile** shows the same steps.
- The first-run guide for admins has a new optional **Start Reading delivery** step.

### 👪 Kids' accounts
- **One-click presets** on each kid's card: **👪 Strict Family (Max Level 2)** and the new **🧒 Young Reader (Level 1, ages 9–12)**, which also sets the age group and hides dark occult. Rules a preset doesn't cover stay as you set them.
- The pepper limit for kids now offers **No limit** or **Level 0–3**.

### ⚡ Changed books
- **Only changed books are rated again**: NovelCheck now remembers when each book last changed in Calibre (its details or its file). Unchanged books are never rated twice; if you edit a book in Calibre after it was rated, it joins the **Re-rate** banner on the Admin page ("changed in Calibre since they were rated"), so nothing is spent without your say-so. **Re-rate whole library** is still there to redo everything.

### 🧹 Tidier Admin page
- **Four tabs** instead of one long page: **🤖 AI & Scans** (progress, batches, re-rates, custom filters, AI settings), **👪 Users & Rules**, **📬 Delivery & Services** (Send-to-Kindle, Calibre, KOReader, remote access) and **⚙️ System & Toggles** (features, sign-in, backups). Editors see the first two. The page remembers which tab you were on.
- Settings are in **collapsible cards**, each tab has its own **Save**, and the backup AI's details stay hidden until you switch the backup on.
- User cards show a **role badge**, and for kids their **pepper limit** and **age group** at a glance.

### 📈 Usage & Ollama
- **Tap a chart to read it**: on the Usage page, tap (or hover over) any day's bar or any point on a line to see the exact date and value. On a phone it stays shown until you tap somewhere else.
- **🧹 Installed models**: the Ollama easy setup has a new button listing every model on your Ollama server with the disk space it uses, biggest first, and a **Delete** button to free space. Models NovelCheck is set to use are marked **In use** and can't be deleted by accident.

### 🏷️ Tidy titles
- **Clean titles everywhere**: titles with track or series numbers, such as "01 - The Hobbit", "Book 2: Catching Fire" or "Guards! Guards! (Discworld, #8)", now show as the plain title, with the series and number underneath (**📚 Discworld #8**). Calibre's own series always wins. Titles like "1984" or "13 Reasons Why" are left alone. Library search finds series names too.
- **Fix them in Calibre in one click**: the Admin page lists every numbered title (**🏷️ Tidy in Calibre**). Tick the ones to change and NovelCheck saves the tidy titles in Calibre through its Content server. It first checks that each book matches, so the wrong library is never touched. An admin can also edit any book's title, series and number from its window (**✏️ Edit title in Calibre**). There's an **Open in Calibre-Web** link if the Content server isn't set up.

### Fixed
- **Renaming a book in Calibre keeps its rating**: a book renamed in Calibre (or moved to a new folder) used to come back as a new, unrated book next to the old one. It now stays the same book, with its rating, notes and Up Next places, and its file links follow the move. Tidying a title doesn't put the book in the re-rate list.
- **No more error when you reopen the app**: when a phone wakes NovelCheck before its connection is back, the app now waits for the network and quietly tries again (up to 3 times) instead of showing an error.
- **Steadier server**: if something goes wrong while rating a book, syncing Calibre or sending a notification, NovelCheck now records the error and carries on instead of stopping (which made the app restart).
- **Books per batch**: **0** now means "all waiting books" (up to 500), a cleared box goes back to 20, and anything else invalid is refused with a clear message.
- **Looked-up books can't be deleted**: a book you only looked up with Check a book isn't in your library, so its window no longer offers **Request to delete**.
- **Photos on Check a book**: a new **Choose a photo you already took** button, and a clear message when a photo can't be opened.

### Changed
- **Plain pepper labels**: ratings now read **Level 0–5** with their name (for example "Level 2: Mild / Closed Door"), without nicknames. Level 3 is called **Steamy Closed Door / Heavy Tension**. Books at Level 3 or higher show the AI's reason as a tag, with a "why" tooltip.
- Neutral wording in the import guides and examples.

## [1.16.0]

### 📷 Check a book
- **Snap a cover, see the rating**: the new first tab for parents, **📷 Check a book**, is where NovelCheck now opens. In a shop, tap **Take a photo of the cover** (or type the title, author or ISBN) and you get a plain answer: "✓ Level 0–2", "⚠ Level 3" or "✕ Level 4–5", plus the peppers, why, the tags and a short summary.
- Books already in your library answer at once. New ones are looked up and rated on the spot (usually 10–30 seconds) and kept under **Looked up**, so checking them again is instant and free. **Recently checked** lists the last few.
- Your AI reads the title from the photo; most cloud AIs can (OpenAI, Claude, Gemini). With a small local Ollama model that can't read images, type the title instead. On Android, the barcode on the back is read by the phone itself.

### Changed
- **Clearer pepper levels 2 and 3**: **Level 2: Mild / Closed Door** now means romantic tension and kissing, with anything more happening off the page. **Level 3: Steamy Closed Door / Heavy Tension** means heavy foreplay or suggestive scenes on the page that stop short of explicit. Levels 4 and 5 are now called **Explicit / Open Door** and **Very Explicit / Erotica**.
- **Re-rate with the new wording**: books the AI rated before this change show up in Admin as "rated with older pepper rules", with a **Re-rate with the current rules** button and a cost estimate. They stay in the library with their old rating until the new one arrives, and hand-rated books are left alone.
- **Phones: search first**: on a phone the Library shows just the search box and a **Filters** button, so books appear right away. The button shows how many filters are on, like **Filters (2)**.

### New
- **Why this many peppers**: every new rating includes a few words on what set the level, like "Kissing only" or "Heavy innuendo, on-page foreplay". It's shown in each book's window, and you can edit it under **Edit rating**.
- **Why tag**: books at Level 3 show a tag on their card saying why, so parents can tell at a glance.
- **👪 Strict family preset**: one click on a kid's account allows at most 2 peppers and hides Open Door, nudity, solo acts, heavy innuendo and books that haven't been rated yet.
- **Re-rate whole library**: a button on the Admin page rates every AI-rated book again (for example after switching to a better AI), with a cost estimate first.
- **Open in Calibre-Web**: if you use Calibre-Web, enter its address under **Admin → Calibre Library**. Each book's window then has an **Open in Calibre-Web ↗** link next to its Calibre ID, for admins and editors.
- **Turn off what you don't use**: **Admin → Features** has switches for the Up Next reading queue, Send-to-Kindle email, KOReader sync, Import books, and parent tools (kids' accounts, age groups, parents' notes). A feature that's off disappears for everyone. Kids' content rules always keep working, even with parent tools hidden.
- **More in the 🔔 bell**: it now also tells you when a Calibre sync adds new books, when a batch of ratings finishes (how many books, tokens and cost), and when someone starts reading a book (and how it was sent). You can turn these everyday notices off under **Admin → Features**.
- **Hourly AI limit notice**: when rating pauses because the hourly token limit is reached, the 🔔 bell says so, and the notice clears itself when rating continues.
- **Your own AI filters**: under **Admin → Custom AI filters**, add topics your family cares about, like *Heavy swearing*, *Gore / violence* or *Substance abuse*, with a sentence on what the AI should look for (up to 12). The AI checks every book for them, matching books get a tag, the Library gets a **Hide** box for each, and **Edit rating** lets you tick them by hand. After adding a filter, the Admin banner offers to re-rate existing books so they're checked too.
- **📱 Phone notifications**: under **Profile → Phone notifications**, tap **Turn on for this device** to get notices on your phone or computer even when NovelCheck is closed. Parents get the 🔔 notices (or **Only problems**); everyone gets "📚 Ready to read" when a book they started is sent to their Kindle or KOReader. **Send a test** checks it works. Notifications need NovelCheck's secure **https://** address (set up under **Admin → Remote access**). On iPhone and iPad, first add NovelCheck to the Home Screen (Share → Add to Home Screen) and open it from there.
- A thank-you to Calibre and Calibre-Web in the footer.

## [1.15.0]

### New
- **🗑 Request to delete**: anyone can open a book and ask for it to be deleted, with an optional reason (and cancel it later). Books with a request show a **🗑 Delete requested** tag.
- **Delete requests page** (admins): **Library → Delete requests** (or the banner on Admin, or the 🔔 bell) lists each requested book with who asked and why. Tick books and **Delete from Calibre** (to Calibre's recycle bin, with the same safety checks as duplicates), **Keep**, or **Mark done** for books that aren't in Calibre. A **Recently handled** list keeps the history.
- **Language for summaries**: **Admin → LLM Analysis Engine → Language for book summaries** (default **English (US)**). The AI now writes every summary in that language, even for books whose description is in another language. If some existing summaries aren't in English, Admin offers **Re-rate them in English**; they stay in the library while that happens.
- **Import from Amazon, Goodreads, StoryGraph, Hardcover or a spreadsheet**: the Import page (now called **Import books**) takes a list file with no Kindle cable. NovelCheck finds the title, author and ISBN columns by itself, lets you pick Goodreads shelves or StoryGraph statuses, and removes Goodreads' series tags so books match your Calibre library. Step-by-step guides explain how to get the file from each service.
- **Amazon library in one paste**: copy Amazon's **Content Library** page and use **Paste a list**. NovelCheck picks out each title and author and skips the buttons and dates. Remove anything that isn't a book with ✕ before importing.

### Changed
- The 🔔 bell refreshes when you move between pages, so new notices appear sooner.

## [1.14.2]

### Fixed
- **No way to confirm duplicates were removed**: after **Remove selected copies**, NovelCheck now re-reads your Calibre library before answering, reloads the list, and says what happened (for example "✓ Removed 12 copies. Calibre now has no duplicates"). A new **🔄 Check again** button re-reads Calibre and refreshes the list any time, with the time it last read Calibre.
- **Duplicates missed for entries without files**: two Calibre entries of the same book that had no book files weren't recognised as duplicates. They are now (after the next Calibre read, or **Check again**).

## [1.14.1]

### Fixed
- **Errors you couldn't see or retry**: the Errors count on Usage didn't say what went wrong, and failed books were never tried again. A new **Rating errors** list (on Usage, and on Admin for editors too) groups failed books by reason, shows example titles and a plain explanation of each common error, and has **Retry** buttons for one group or **Retry all**. You can also **📋 Copy** or **🩺 Diagnose** a group.
- **Library → Rating failed**: the pepper filter can now show just the books that failed; **Show in Library** in the errors list opens it.

## [1.14.0]

### New
- **Model recommendations for your GPU**: the Ollama easy setup now works out how much GPU memory your Ollama server has and labels every model: **⭐ Best for your GPU**, **💪 Most powerful that fits**, **✓ Fits**, or **⚠️ Too big (slow)**. Without a GPU it recommends the small models and tells you if Ollama is running on the CPU only.
  - **🎮 Check my GPU** loads your biggest downloaded model for a moment to measure (Ollama can't report the GPU directly), then unloads it. Or just pick your GPU's memory size from the menu.
  - Downloaded models that are too big for the GPU get a **⚠️ slow** tag in the model order list.
  - New larger choices in the download menu: Qwen 2.5 3B, Gemma 3 12B, Qwen 2.5 14B, Gemma 3 27B and Qwen 2.5 32B.

## [1.13.0]

### New
- **Backup AI**: a new **Admin → Backup AI (optional)** section sets up a second AI that takes over when the main one fails: another Ollama on your network (the **Find Ollama** helper works here too) or a cloud AI like OpenAI or Claude. If the main server is switched off, NovelCheck goes straight to the backup instead of waiting on each model. The 🔔 bell tells you when the backup was used, and each AI's cost is counted at its own prices.
- **Public address check**: **Check everything** now loads your remote-access address from the internet and confirms NovelCheck answers, not just that the tunnel is connected. It explains what to fix (missing Public Hostname, wrong service address), or tells you the problem is your device's network when the site works from outside.

### Changed
- **Clearer "AI server is off" message**: Check everything says "Can't reach the AI server" with what to check, instead of a technical "dial tcp" error. AI diagnosis also uses the backup AI when the main one is down.

## [1.12.0]

### New
- **🩺 Diagnose with AI** (System page, admins): your connected AI reads NovelCheck's diagnostics (settings, recent problems, the worker and the remote-access log, never passwords or keys) and explains in plain words what's wrong and how to fix it. It also tells you whether it looks like a bug or something you can fix yourself.
- **Bug reports for GitHub**: every diagnosis comes with a ready-made bug report. **Open GitHub issue** opens the NovelCheck bug form with it filled in (and copies it, in case it's too long). Your web address is swapped for a placeholder. If the AI itself is what's broken, you still get the report with the diagnostics.
- **🩺 Diagnose shortcuts**: error pop-ups, 🔔 notifications and failed books have a **🩺 Diagnose** button that opens the diagnosis with the problem filled in.
- **Easy copying**: **📋 Copy** and **⬇ Download** on the Cloudflare connector log (now bigger, and it no longer loses your selection when it refreshes), the **Check everything** results, each notification (**📋 Copy all** too), error pop-ups and failed books. **Copy diagnostics** / **Download diagnostics** give the raw report.

## [1.11.1]

### Fixed
- **"context deadline exceeded" with Ollama**: NovelCheck gave every book only 2 minutes, which is too short on your own hardware while Ollama loads a model or when it runs on the CPU. The limit is now automatic: **10 minutes** for AI on your own network (Ollama, LM Studio…) and 2 minutes for cloud services. You can set your own under **Admin → LLM Analysis Engine → AI time limit per book**.
- **Clearer message when the AI is too slow**: it now names the model and suggests what to check (Usage → Ollama should say *100% GPU*; otherwise try a smaller model).
- **Check everything** waits up to 90 seconds for a local model to load instead of 10.

## [1.11.0]

### New
- **🌶️ The pepper scale**: books are now rated **0 to 5 peppers**: 0 No Romance, 1 Sweet Romance, 2 Romantic, 3 Steamy Closed-Door, 4 Explicit, 5 Very Explicit / Erotica-Level. The AI uses these exact descriptions and examples. **What do the peppers mean?** in the Library (and **About peppers** in each book) shows them all.
- **Pepper filter**: the Library can show books with a given number of peppers, older ratings, or books not rated yet.
- **Most peppers allowed** for kid accounts: new kids start at 0 (Young kids), 1 (Middle grade), 2 (Teens), 3 (Young adult) or no limit (Adults), and you can change it under **Admin → Users**. Existing kid accounts start with no limit, so nothing changes for them until you set one.
- **Edit rating** now asks for peppers.
- **Re-rate older books**: books rated before the pepper scale keep their old label (No Spice, Closed Door, Open Door) and show a **Re-rate them on the pepper scale** button in Admin. They stay in the library while they're re-rated, and hand-rated books are left alone. Until then a kid's pepper limit plays it safe: an old "No Spice" counts as up to 2 peppers, "Closed Door" as 3 and "Open Door" as 4.

## [1.10.0]

### New
- **Put your Ollama models in order**: the Ollama easy setup lists every downloaded model. Tick the ones to use and move them with **↑ / ↓**. **#1** rates every book; if it fails on a book, **#2** tries, then **#3**, and so on. One click saves them all, with no copy-paste.
- **Several fallback models for any provider**: the **Fallback model(s)** box takes a comma-separated list, tried in order. **Check everything** tests each one.

## [1.9.1]

### Fixed
- **"Send test email" tested the old password**: it used the last *saved* settings, so a new password typed without clicking **Save settings** was never tried. It now tests exactly what's on screen.
- **Browser filling in the wrong password**: Chrome and password managers could fill your NovelCheck login into the email username/password fields. Those fields are now marked so browsers leave them alone.
- **Gmail App Passwords with spaces**: the spaces Google shows ("abcd efgh ijkl mnop") are removed automatically.
- **Clearer Gmail error**: when Gmail rejects the login, NovelCheck now explains that Gmail needs an App Password (not your normal password) and your full Gmail address as the username. The SMTP box links straight to Google's App Password page.

## [1.9.0]

### New
- **Find duplicates**: **Library → Find duplicates** lists books that are in Calibre more than once (same title and author, even if the author is spelled slightly differently). For each copy it shows the Calibre number, formats and file size, and suggests which copy to keep: the best formats (EPUB first), then the most files, then the largest.
- **Remove duplicates in one click** (admins, with **One-click removal** turned on): tick the extra copies and click **Remove selected copies**. They go to Calibre's recycle bin, and NovelCheck never lets you remove every copy of a book. Without one-click removal, **Copy Calibre search** gives you a search to paste into Calibre. Editors can view the list.
- **File formats everywhere**: book cards show their formats (EPUB, AZW3, MOBI…) and a **⚠ 2× in Calibre** tag for duplicates. The book window lists each Calibre copy with all of its formats.
- **Format filter**: the Library can show only EPUB, AZW3, MOBI, KFX or PDF books, books with **2+ formats**, **Duplicates**, or books with **No file**.

## [1.8.1]

### Fixed
- **Ollama address without `http://`**: typing an address like `10.13.6.41:30068` broke book rating ("first path segment in URL cannot contain colon"). NovelCheck now fills in `http://` and Ollama's `/v1` for you, including for an address you already saved.
- **Find Ollama button missing**: with the TrueNAS Ollama app's port (30068), the AI provider showed as "Other" and hid the Ollama easy setup. Port 30068 is now recognized, and your saved address is filled in for **Find Ollama**.

### New
- **Kindles that show up as a "device"**: newer Kindles connect like a phone, which browsers can't open. The Import page now explains how to copy the Kindle's **documents** folder to the Desktop and import that instead.
- **Paste a list**: on the Import page you can type or paste books, one per line ("Title by Author", "Title - Author", or just the title), with no cable needed.

## [1.8.0]

### New
- **Age groups**: open any book and set its **Age group**: Young kids (up to 8), Middle grade (9–12), Teens (13–15), Young adult (16–17) or Adults (18+). Choosing one also counts as rating the book, even before the AI gets to it. The Library has a **Suitable for…** filter, and book cards show the age group.
- **Kid accounts by age group**: when you add a kid, pick their age group ("Kid · Teens (13–15)", for example). Each group starts with sensible content rules, and kids only see books rated for their group or younger. Existing kid accounts can be switched under **Admin → Users**.
- **Parents' notes**: after reading a book, leave a note on it. Notes can be for **everyone** (kids see them as "Notes from your parents") or **parents only** 🔒. You can edit or delete your own notes.
- **Usage tab** (admins): CPU, memory, **network speed** (download and upload) and disk, with 15-minute charts, plus books rated, AI calls and cost, AI tokens per day for the last two weeks, and what Ollama has loaded. The **System** tab now holds just the connection checks.

## [1.7.0]

### New
- **System page** (admins): a new **System** tab shows NovelCheck's CPU, memory, disk space and database size, and, when you use Ollama, which models are loaded and how much sits in GPU memory versus system RAM.
- **Check everything**: one button tests every connection NovelCheck uses (your AI provider and fallback model, Open Library, Google Books, the Calibre library and Content server, email, remote access, Ollama, and the update check) and says how to fix anything that fails.
- **Notifications 🔔**: admins and editors get a bell with a count. It lists problems NovelCheck notices on its own (books failing to rate, Calibre sync errors, remote access dropping, Send-to-Kindle failures, Ollama downloads failing, failed checks) with **Fix it** links, **Mark read** and **Clear all**. Some clear themselves once the problem is fixed.
- **Google Books API key guide**: without a key, Google's shared free quota often runs out. The AI setup guide now explains how to get a free key.

### Fixed
- **Error pop-ups vanished too fast**: error messages now stay until you close them (✕), and every message is also listed under the bell in "Messages on this device".

## [1.6.0]

### New
- **One-click removal from Calibre** (admins): connect NovelCheck to Calibre's Content server under **Admin → Calibre Library → One-click removal** (**Show setup steps** explains the Calibre side). Then **Remove hidden books from Calibre…** has a **Remove these books** button. Books go to Calibre's recycle bin, and books marked OK are never included.
- **Safety checks**: before removing anything, NovelCheck re-checks the list and confirms every title with Calibre. If Calibre is serving a different library, it stops and removes nothing.

## [1.5.0]

### Changed
- **Filter boxes now hide books**: the Library's checkboxes are labeled **Hide:**. Ticking **Nudity**, for example, hides books with nudity instead of showing only those. There's also a new **Open Door** box.

### New
- **✓ Mark as OK**: if a filter catches a book that's fine for your family (Harry Potter's magic, say), an admin or editor can open it and click **Mark as OK**. It then shows for everyone, kids included, whatever the filters or content rules say. Click **Remove OK mark** to undo.
- **Remove hidden books from Calibre** (admins): in the Library, tick the Hide boxes, then click **Remove hidden books from Calibre…**. NovelCheck lists the matching Calibre books (never ones marked OK) and gives you a search to paste into Calibre, which then removes exactly those books using its own recycle bin.
- **Suggest a filter / feedback**: links at the bottom of every page and next to the Hide boxes open a short form on GitHub (a free GitHub account is needed).

## [1.4.0]

### New
- **Remote access built in**: use NovelCheck away from home through a free Cloudflare Tunnel, with no router changes. Paste the tunnel token from Cloudflare into **Admin → Remote access**, tick **Turn on remote access**, and you get an `https://` address that works anywhere. **Show setup steps** walks through the Cloudflare side.
- **Ollama easy setup**: pick **Ollama** as the AI provider, click **Find Ollama**, **Download** a model (with a progress bar), and **Use this model**. No terminal commands needed. Works great with GPU passthrough.

## [1.3.2]

### Fixed
- **Calibre sync failed with "no such column: b.isbn"** on libraries made by current Calibre versions, which no longer have that column. ISBNs are now read only from Calibre's identifiers, which every version has. After updating, click **Sync Calibre now**.

## [1.3.1]

### New
- **Setup guides for every AI provider**: in **Admin → LLM Analysis Engine**, click **Show setup steps** for click-by-click instructions for the selected provider (creating an account, getting an API key, what to paste where, typical costs), or **Which one should I pick?** for a quick comparison. The same guide is in `docs/AI_PROVIDERS.md` on GitHub.

## [1.3.0]

### New
- **Choose your AI provider**: **Admin → LLM Analysis Engine** has an **AI provider** menu with OpenAI, **Anthropic Claude**, Google Gemini, Perplexity, and Ollama (free, on your own server). Picking one fills in the address, a low-cost model, and prices, and says where to get an API key.
- **Claude support**: NovelCheck talks to Claude directly through Anthropic's official API. By default Claude Haiku 4.5 rates books, and Claude Sonnet 5 is only used when Haiku can't.
- **Perplexity**: its Sonar model can search the web, which helps with lesser-known and self-published books.

## [1.2.1]

### Fixed
- **Updates not arriving on TrueNAS**: the install guide and `docker-compose.yml` now set the image pull policy to **Always**. Without it, TrueNAS keeps reusing its old download of `latest`. If you installed earlier, edit the app once and set it (see "Updating NovelCheck" in the TrueNAS guide).

### New
- **TrueNAS "Install Custom App" steps**: the install guide now covers the TrueNAS form as well as YAML.

## [1.2.0]

### New
- **Stay signed in**: the sign-in screen has **Keep me signed in on this device** (on by default). You stay signed in as long as you use NovelCheck at least once every 30 days. Admins can change the number of days under **Admin → Sign-in & Updates**. Untick it on shared computers: you're signed out when the browser closes.
- **Version in the header**: the running version now shows at the top right next to **Sign out**. Click it to see what's new.

## [1.1.0]

### New
- **Create the admin account in the browser**: new installs open a "Welcome to NovelCheck" page to set the admin username and password. There's no password in the TrueNAS YAML any more (`NOVELCHECK_ADMIN_PASSWORD` still works for automated installs).
- **How-to guide**: a short step-by-step guide opens the first time each person logs in, tailored to their account type. Reopen it any time from **❔ Help** at the top or **How to use NovelCheck** at the bottom.
- **Editor role**: a middle account type between Admin and Kid. Editors can review the dashboard and costs, run scans and Calibre syncs, import drives, correct book ratings by hand, and manage kids' accounts. They can't change AI or email settings, API keys, the library folder, or backups, and can't touch admin accounts.
- **Edit rating**: admins and editors can correct a book's spice level, content flags and summary from the book window. Hand-made ratings are labeled with who changed them.
- **Choose the Calibre library folder in the app**: mount a parent folder (for example your whole media share), then in **Admin → Calibre Library** browse to the exact library or let NovelCheck find it automatically.
- **Update notices and What's new**: admins and editors see a banner when a new version is out, and everyone can read the release notes in the app. Admins can turn update checks off.

### Changed
- Existing databases upgrade automatically on first start. No action needed.
- The TrueNAS guide now mounts a parent folder (for example your whole `plex` share) and picks the library inside NovelCheck.
- Existing users will see the How-to guide once on their next login.

## [1.0.0]

First release: Calibre sync (read-only), browser drive/Kindle scanner, spice and content analysis with any OpenAI-compatible AI, parental content rules, Up Next reading queue with Send-to-Kindle, admin panel with cost controls, installable phone app, and a TrueNAS install guide.
