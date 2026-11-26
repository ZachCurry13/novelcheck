/* NovelCheck Kindle app: a book's page. The cover, the spice level with its door, the verdict, the heads-up the
   person asked for, what the book contains (as on the web page), genres, Hardcover readers' stars, the summary; and
   Download, Add to Nightstand, and "I finished it" with stars (the plugin asks at the end of a book; Amazon's reader
   can't, so it is here). ES5 only. */

function details(id) {
  note("Loading…");
  api("GET", "/kosync/plugin/info/" + id, null, function (err, b, status) {
    if (err) {
      var c = cachedInfo(id);
      if (!c) return note(err);
      if (status === 0) setOffline(true, c.at);
      note("");
      return paintDetails(c.b); // the page as it was saved
    }
    setOffline(false);
    if (b && b.pending) return note("Still being checked. Try again in a minute.");
    if (b && b.blocked) return paintBlocked(b);
    cacheInfo(id, b);
    note("");
    paintDetails(b);
  });
}

function paintDetails(b) {
  var id = b.id;
  {
    showLists(false);
    backTo = function () { rows(); };
    paintNav("Book");
    window.scrollTo(0, 0);
    var m = $("main");
    m.innerHTML = "";
    $("pages").hidden = true;
    var d = el("div", "book");
    var img = el("img");
    d.appendChild(img);
    cover(img, id, "l");
    d.appendChild(el("h2", "", b.title));
    d.appendChild(el("div", "by", b.author + (b.series ? " · " + b.series + (b.series_index ? " #" + b.series_index : "") : "")));
    d.appendChild(el("p", "title", (b.level == null ? "Not rated yet" : "Spice level " + b.level + (b.door ? " · " + b.door : "")) +
      (b.reason ? ": " + b.reason : "") + (b.old_rules ? " (older rating)" : "")));
    if (b.heads_up && b.heads_up.length) d.appendChild(el("p", "headsup", "Heads-up: " + b.heads_up.join(" · ")));
    if (b.verdict) d.appendChild(el("p", "", b.verdict));
    var facts = [];
    if (b.flags && b.flags.length) facts.push("Has: " + b.flags.join(", "));
    for (var i = 0; b.content && i < b.content.length; i++) facts.push(b.content[i].group + ": " + b.content[i].items.join(", "));
    if (facts.length) {
      var ul = el("ul", "facts");
      for (var j = 0; j < facts.length; j++) ul.appendChild(el("li", "", facts[j]));
      d.appendChild(ul);
    }
    var extra = [];
    if (b.genres && b.genres.length) extra.push(b.genres.join(", "));
    if (b.hc_rating > 0) extra.push("★ " + b.hc_rating.toFixed(1) + " from " + b.hc_ratings + " Hardcover readers");
    if (b.my_stars > 0) extra.push("You gave it " + b.my_stars + " ★");
    if (extra.length) d.appendChild(el("p", "meta", extra.join(" · ")));
    if (b.summary) d.appendChild(el("p", "", b.summary));
    var acts = el("div", "actions");
    var inLib = b.in_library !== false && !b.rating_only; // older saved pages don't say: they are library books
    if (b.rating_only) d.appendChild(el("p", "headsup", b.message));
    else if (!inLib && !b.url) d.appendChild(el("p", "headsup", "Not available in the library yet."));
    if (b.url) acts.appendChild(downloadButton(b, true));
    if (b.free && !b.url && inLib === false) { // public domain: one tap puts the free copy in the library
      var fr = el("button", "main", "Add the free copy to the library");
      fr.onclick = function () { fr.disabled = true; addFree(b, fr); };
      acts.appendChild(fr);
    }
    var fmts = { ebook: "ebook", audio: "audiobook" };
    var asks = b.request || [];
    for (var a1 = 0; a1 < asks.length; a1++) (function (f) {
      var rq = el("button", a1 === 0 && !b.url ? "main" : "", "Request the " + fmts[f]);
      rq.onclick = function () { rq.disabled = true; askFor(b, f); };
      acts.appendChild(rq);
    })(asks[a1]);
    var wished = b.wished || [];
    for (var a2 = 0; a2 < wished.length; a2++) {
      var done = el("button", "", "Asked for the " + fmts[wished[a2]]);
      done.disabled = true;
      acts.appendChild(done);
    }
    var onNs = !!b.queue; // the button adds the book to the Nightstand, or takes it off again
    var ns = el("button", "", onNs ? "Remove from Nightstand" : "Add to Nightstand");
    ns.onclick = function () {
      ns.disabled = true;
      api("POST", "/kosync/plugin/nightstand" + (onNs ? "/remove" : ""), { book_id: b.id }, function (err2) {
        ns.disabled = false;
        if (err2) return note(err2);
        onNs = !onNs;
        ns.textContent = onNs ? "Remove from Nightstand" : "Add to Nightstand";
        note(onNs ? "Added to your Nightstand." : "Taken off your Nightstand.");
        if (!onNs) dropFromNightstand(b.id);
        logLine((onNs ? "added to" : "removed from") + " the Nightstand: " + b.title);
      });
    };
    if (inLib) acts.appendChild(ns);
    if (!inLib) { d.appendChild(acts); m.appendChild(d); return; }
    var cv = el("button", "", "Change cover");
    cv.onclick = function () { pickCover(b); };
    acts.appendChild(cv);
    var hide = el("button", "", "Don't show this book");
    hide.onclick = function () { // two taps, so it isn't done by accident
      if (hide.textContent !== "Tap again to hide it") {
        hide.textContent = "Tap again to hide it";
        note("This hides it from your own lists only. It stays in NovelCheck for everyone else, and the web app's Settings can show it again.");
        return setTimeout(function () { hide.textContent = "Don't show this book"; }, 6000);
      }
      api("POST", "/kosync/plugin/left", { book_id: b.id, choice: "hide" }, function (err3) {
        if (err3) return note(err3);
        dropBook(b.id);
        logLine("hidden from my lists: " + b.title);
        rows();
        note("Hidden. It won't show in your lists.");
      });
    };
    acts.appendChild(hide);
    var fin = el("button", "", "I finished it");
    fin.onclick = function () { finishPanel(b, d); };
    acts.appendChild(fin);
    d.appendChild(acts);
    m.appendChild(d);
  }
}

// finishPanel asks for stars (or none) and marks the book finished in NovelCheck (and so on Hardcover).
function finishPanel(b, d) {
  var p = el("div", "actions");
  p.appendChild(el("span", "meta", "Stars: "));
  for (var n = 0; n <= 5; n++) {
    (function (stars) {
      var s = el("button", "", stars ? stars + "★" : "None");
      s.onclick = function () {
        api("POST", "/kosync/plugin/finish", { device: NC.device, title: b.title, author: b.author, filename: b.title, stars: stars }, function (err, r) {
          if (err) return note(err);
          logLine("finished: " + b.title + " (" + stars + " stars)");
          note("Marked finished" + (stars ? " with " + stars + " ★" : "") + "." + (r && r.next ? " Next in the series: " + r.next.title + "." : ""));
          d.removeChild(p);
        });
      };
      p.appendChild(s);
    })(n);
  }
  d.appendChild(p);
}

// pickCover: the covers NovelCheck found for the book, two across; tapping "Use this" makes it your cover on every
// screen (and, once the book is downloaded, its tile in the Kindle's library). "Back to the usual cover" undoes it.
function pickCover(b) {
  note("Looking for covers…");
  api("GET", "/kosync/plugin/cover-options/" + b.id, null, function (err, data) {
    if (err) return note(err);
    var list = (data && data.candidates) || [];
    note(list.length ? "" : "No other covers found for this book.");
    showLists(false);
    backTo = function () { details(b.id); };
    paintNav("Choose a cover");
    window.scrollTo(0, 0);
    var m = $("main");
    m.innerHTML = "";
    $("pages").hidden = true;
    var box = el("div", "cands");
    for (var i = 0; i < list.length; i++) {
      (function (c) {
        var d = el("div", "cand");
        var img = el("img");
        img.alt = "";
        d.appendChild(img);
        coverFrom(img, "/kosync/plugin/cover-preview?u=" + encodeURIComponent(c.url));
        var use = el("button", "", "Use this");
        use.onclick = function () {
          use.disabled = true;
          use.textContent = "Saving…";
          api("POST", "/kosync/plugin/cover", { book_id: b.id, url: c.url }, function (e2) {
            if (e2) { use.disabled = false; use.textContent = "Use this"; return note(e2); }
            coverVer[b.id] = new Date().getTime();
            logLine("cover chosen for " + b.title);
            note("Cover changed.");
            details(b.id);
          });
        };
        d.appendChild(use);
        box.appendChild(d);
      })(list[i]);
    }
    m.appendChild(box);
    var reset = el("button", "", "Back to the usual cover");
    reset.onclick = function () {
      api("POST", "/kosync/plugin/cover", { book_id: b.id, reset: true }, function (e3) {
        if (e3) return note(e3);
        coverVer[b.id] = new Date().getTime();
        note("Back to the usual cover.");
        details(b.id);
      });
    };
    var tail = el("div", "actions");
    tail.style.clear = "both";
    tail.appendChild(reset);
    m.appendChild(tail);
  });
}

// dropFromNightstand takes a book out of the Nightstand list on screen and in the saved copy, so it does not come back
// until the next refresh says so.
function dropFromNightstand(id) {
  var keep = [];
  var saved = cachedList("up-next");
  if (saved && saved.books) {
    for (var i = 0; i < saved.books.length; i++) if (saved.books[i].id !== id) keep.push(saved.books[i]);
    cacheList("up-next", keep, saved.header);
  }
  if (state.list === "up-next") {
    var rest = [];
    for (var j = 0; j < state.books.length; j++) if (state.books[j].id !== id) rest.push(state.books[j]);
    state.books = rest;
  }
}

// dropBook takes a hidden book out of every list: the one on screen and the saved copies of all of them.
function dropBook(id) {
  var keepOnly = function (books) {
    var out = [];
    for (var i = 0; i < books.length; i++) if (books[i].id !== id) out.push(books[i]);
    return out;
  };
  for (var key in LISTS) {
    var saved = cachedList(key);
    if (saved && saved.books) cacheList(key, keepOnly(saved.books), saved.header);
  }
  state.books = keepOnly(state.books);
}
