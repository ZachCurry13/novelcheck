/* NovelCheck Kindle app: a book's page. The cover, the spice level with its door, the verdict, the heads-up the
   person asked for, what the book contains (as on the web page), genres, Hardcover readers' stars, the summary; and
   Download, Add to Nightstand, and "I finished it" with stars (the plugin asks at the end of a book; Amazon's reader
   can't, so it is here). ES5 only. */

function details(id) {
  note("Loading…");
  api("GET", "/kosync/plugin/info/" + id, null, function (err, b) {
    if (err) return note(err);
    note("");
    window.scrollTo(0, 0);
    var m = $("main");
    m.innerHTML = "";
    $("pages").hidden = true;
    var d = el("div", "book");
    var img = el("img");
    d.appendChild(img);
    cover(img, id);
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
    if (b.url) acts.appendChild(downloadButton(b, true));
    var ns = el("button", "", b.queue ? "On your Nightstand" : "Add to Nightstand");
    ns.disabled = !!b.queue;
    ns.onclick = function () {
      api("POST", "/kosync/plugin/nightstand", { book_id: b.id }, function (err2) {
        if (err2) return note(err2);
        ns.textContent = "On your Nightstand";
        ns.disabled = true;
        logLine("added to the Nightstand: " + b.title);
      });
    };
    acts.appendChild(ns);
    var fin = el("button", "", "I finished it");
    fin.onclick = function () { finishPanel(b, d); };
    acts.appendChild(fin);
    var back = el("button", "", "Back");
    back.onclick = rows;
    acts.appendChild(back);
    d.appendChild(acts);
    m.appendChild(d);
  });
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
