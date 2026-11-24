/* NovelCheck Kindle app: a list fits the screen. Instead of scrolling (slow on e-ink), each page holds as many books as
   fit, and Prev / Next turn the page. After every drawing, the number is adjusted until the content just fits the
   window: one fewer while it runs past the bottom, one more while another book still fits. Only a book's own page
   scrolls (with ▲ ▼). ES5 only. */
var fitting = false;

// bottomOfList is where the list's content ends on screen: the page buttons when they show, else the last book.
function bottomOfList() {
  var el = !$("pages").hidden ? $("pages") : null;
  if (!el) { var rows = document.querySelectorAll("#main .row"); el = rows.length ? rows[rows.length - 1] : $("main"); }
  return el.getBoundingClientRect().bottom + (window.pageYOffset || 0);
}

function fit() {
  var h = window.innerHeight || 0;
  if (fitting || h < 400) return;
  fitting = true;
  var first = state.page * PER_PAGE, tries = 0, room = 12; // a little air under the last thing
  var redraw = function (n) {
    PER_PAGE = n;
    state.page = Math.floor(first / PER_PAGE);
    rows();
  };
  while (tries++ < 14 && bottomOfList() + room > h && PER_PAGE > 2) redraw(PER_PAGE - 1); // too tall: one book fewer
  // room for more: add a book while one more still fits (a book is about as tall as the rows drawn so far on average)
  var drawn = document.querySelectorAll("#main .row");
  if (drawn.length && shown().length > PER_PAGE && tries < 28) {
    var avg = (drawn[drawn.length - 1].getBoundingClientRect().bottom - drawn[0].getBoundingClientRect().top) / drawn.length;
    while (tries++ < 28 && PER_PAGE < 30 && shown().length > PER_PAGE && bottomOfList() + avg * 0.9 + room < h) {
      redraw(PER_PAGE + 1);
      if (bottomOfList() + room > h) { redraw(PER_PAGE - 1); break; } // that one did not fit after all
    }
  }
  fitting = false;
}
