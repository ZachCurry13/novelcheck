/* NovelCheck Kindle app: a list fits the screen. Instead of scrolling (slow on e-ink), each page holds as many books as
   fit, and Prev / Next turn the page. After every drawing, the number is adjusted until the whole page fits the window;
   only a book's own page scrolls (with ▲ ▼). ES5 only. */
var fitting = false;

function fit() {
  var h = window.innerHeight || 0;
  if (fitting || h < 400) return;
  fitting = true;
  var first = state.page * PER_PAGE, tries = 0;
  var over = function () { return document.body.scrollHeight - h; };
  var redraw = function (n) {
    PER_PAGE = n;
    state.page = Math.floor(first / PER_PAGE);
    rows();
  };
  while (tries++ < 14 && over() > 4 && PER_PAGE > 2) redraw(PER_PAGE - 1); // too tall: one book fewer
  var rowH = (document.querySelector(".row") || {}).offsetHeight || 0;
  if (rowH && over() < -rowH && shown().length > PER_PAGE) { // room for more: add books until one would not fit
    while (tries++ < 28 && PER_PAGE < 30 && over() < -rowH && shown().length > PER_PAGE) redraw(PER_PAGE + 1);
    if (over() > 4 && PER_PAGE > 2) redraw(PER_PAGE - 1);
  }
  fitting = false;
}
