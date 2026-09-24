// Step-by-step help for getting a book list out of other apps and services,
// written for people who have never exported anything before.

const guide = (title, steps, note = "") => `
  <details class="rounded-lg bg-slate-800/60 p-3 text-sm text-slate-300">
    <summary class="cursor-pointer font-semibold text-slate-100">${title}</summary>
    <ol class="ml-5 mt-2 list-decimal space-y-1">${steps.map((s) => `<li>${s}</li>`).join("")}</ol>
    ${note ? `<p class="mt-2 text-xs text-slate-400">${note}</p>` : ""}
  </details>`;

export const IMPORT_GUIDES = `
  <div class="space-y-2">
    ${guide("📚 Amazon Kindle books she bought: the quick way (copy the page)", [
      "On a computer, open <b>amazon.com</b> and sign in.",
      "Hover <b>Account &amp; Lists</b> and click <b>Content Library</b> (it used to be called <b>Manage Your Content and Devices</b>), then the <b>Books</b> tab.",
      "If there's a <b>Show</b> or <b>per page</b> option at the bottom, pick the biggest number. Scroll to the bottom of the page so every book has loaded.",
      "Click anywhere on the page, then press <b>Ctrl + A</b> (Mac: <b>⌘ + A</b>) to select everything, and <b>Ctrl + C</b> (⌘ + C) to copy.",
      "Back here, click <b>Paste a list</b> above, click in the box, press <b>Ctrl + V</b> (⌘ + V), then <b>Use this list</b>. NovelCheck picks out each title and author and skips the buttons and dates.",
      "More than one page of books? Repeat for each page. Books already imported are matched, not doubled.",
    ], "Check the list before clicking Import books. If a line looks wrong, you can still import; it only affects how that one book is matched.")}
    ${guide("📦 Amazon: the complete list (Amazon's own data download)", [
      "On amazon.com, go to <b>Account &amp; Lists → Account</b>, then find <b>Request Your Data</b> (search Amazon's help pages for Request Your Data if you can't see it).",
      "Choose the category for <b>Kindle</b> or <b>digital orders</b> (or request everything) and submit. Amazon asks you to confirm by email.",
      "Wait for the email saying your data is ready. It can take a few hours to a few days.",
      "Download the file and open it (it's a .zip: double-click it). Look for a spreadsheet file (.csv) with your book titles, often in a folder with <b>Digital</b> or <b>Kindle</b> in its name.",
      "Here, click <b>Choose a list file</b> below and pick that .csv file.",
    ], "This includes free books. It may also list things that aren't books (apps, music): remove those from the list with ✕ before importing.")}
    ${guide("⭐ Goodreads", [
      "On a computer, open <b>goodreads.com</b> and sign in (the phone app can't export).",
      "Click <b>My Books</b>. In the left column under <b>Tools</b>, click <b>Import and export</b>.",
      "Click <b>Export Library</b> at the top. After a minute, a link <b>Your export from …</b> appears (refresh the page if it doesn't). Click it to download.",
      "Here, click <b>Choose a list file</b> and pick the downloaded file (goodreads_library_export.csv). You can then choose which shelves to import, like <b>read</b> or <b>to-read</b>.",
    ])}
    ${guide("📊 The StoryGraph", [
      "On <b>app.thestorygraph.com</b>, click your profile picture, then <b>Manage Account</b>.",
      "Scroll to <b>Manage Your Data</b> and click <b>Export StoryGraph Library</b>. When it's ready, download the .csv file.",
      "Here, click <b>Choose a list file</b> and pick it. You can choose read statuses to import.",
    ])}
    ${guide("📖 Hardcover", [
      "On <b>hardcover.app</b>, open your profile menu and go to <b>Settings</b>.",
      "Look for <b>Export</b> (under Import &amp; Export or Data) and export your library as a CSV. Download it when it's ready.",
      "Here, click <b>Choose a list file</b> and pick it.",
    ], "Hardcover's menus change from time to time; if the names differ, look for anything that says Export or CSV.")}
    ${guide("🧾 Any other list or spreadsheet", [
      "Put the books in a spreadsheet (Excel, Google Sheets, Numbers) with a <b>Title</b> column and, if you have it, an <b>Author</b> column. An <b>ISBN</b> column helps too.",
      "Save it as <b>CSV</b>: Excel: File → Save As → CSV. Google Sheets: File → Download → Comma-separated values. Numbers: File → Export To → CSV.",
      "Here, click <b>Choose a list file</b> and pick it. Or just copy the titles and use <b>Paste a list</b>.",
    ])}
  </div>`;
