/* NovelCheck Kindle app: look up a book the library does not have (as Check a book does on the web). The search screen
   offers it; a name gives books to pick from; the picked one is saved and rated, and its page opens when the rating is
   ready, with a request button for each format the family lacks. ES5 only. */

function lookupOnline(q) {
  note("Looking it up…");
  api("GET", "/kosync/plugin/lookup?q=" + encodeURIComponent(q), null, function (err, data) {
    if (err) return note(err);
    var choices = (data && data.choices) || [];
    if (!choices.length) return note("No books found.");
    if (data.sure) return checkPick(choices[0]);
    note("");
    showLists(false);
    backTo = function () { show("search", q); };
    paintNav("Which book did you mean?");
    window.scrollTo(0, 0);
    var m = $("main");
    m.innerHTML = "";
    $("pages").hidden = true;
    for (var i = 0; i < choices.length; i++) {
      (function (c) {
        var r = el("div", "row");
        var b = el("button", "act", "Check");
        b.onclick = function () { checkPick(c); };
        r.appendChild(b);
        var what = el("div", "what");
        what.appendChild(el("b", "", c.title));
        what.appendChild(el("div", "meta", c.author + (c.year > 0 ? " · " + c.year : "")));
        r.appendChild(what);
        m.appendChild(r);
      })(choices[i]);
    }
  });
}

// checkPick saves the picked book and has it rated, asking every few seconds until the rating is there (about three minutes
// at most), then opens its page.
function checkPick(c) {
  note("Checking this book. It can take a minute…");
  api("POST", "/kosync/plugin/check", { title: c.title, author: c.author, isbn: c.isbn }, function (err, data) {
    if (err || !data || !data.id) return note(err || "Could not check it.");
    var tries = 0;
    var poll = function () {
      api("GET", "/kosync/plugin/info/" + data.id, null, function (e2, info) {
        tries++;
        if (!e2 && info && info.status !== "analyzed" && tries < 45) return setTimeout(poll, 4000);
        details(data.id);
      });
    };
    poll();
  });
}

// askFor asks for the book in one format (ebook or audio); the page is drawn again to show it was asked for.
function askFor(b, format) {
  api("POST", "/kosync/plugin/wish", { book_id: b.id, format: format }, function (err) {
    if (err) return note(err);
    logLine("asked for " + b.title + " as " + format);
    details(b.id);
    note("Asked for the " + (format === "audio" ? "audiobook" : "ebook") + ". A parent will see it on the wishlist.");
  });
}
