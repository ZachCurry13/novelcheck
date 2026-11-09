// A styled type-ahead for a text box (authors, series): the browser's own <datalist> pop-up is white,
// ignores the app's theme and can't be used with a keyboard on every device. This one matches the app,
// shows the best matches first, and works with the arrow keys, Enter and Esc.
export function combo(input, getOptions, max = 10) {
  const wrap = input.parentElement;
  wrap.classList.add("combo");
  const list = document.createElement("ul");
  list.className = "combo-list";
  list.setAttribute("role", "listbox");
  list.hidden = true;
  wrap.append(list);
  input.setAttribute("role", "combobox");
  input.setAttribute("aria-expanded", "false");
  input.setAttribute("autocomplete", "off");
  let shown = [];
  let at = -1;
  const close = () => {
    list.hidden = true;
    input.setAttribute("aria-expanded", "false");
    at = -1;
  };
  const paint = () => {
    [...list.children].forEach((li, i) => li.setAttribute("aria-selected", String(i === at)));
    list.children[at]?.scrollIntoView({ block: "nearest" });
  };
  const open = () => {
    const q = input.value.trim().toLowerCase();
    const all = getOptions() || [];
    const starts = [];
    const contains = [];
    for (const o of all) {
      const l = o.toLowerCase();
      if (!q || l.startsWith(q)) starts.push(o);
      else if (l.includes(q)) contains.push(o);
      if (starts.length >= max) break;
    }
    shown = [...starts, ...contains].slice(0, max);
    if (!shown.length || (shown.length === 1 && shown[0].toLowerCase() === q)) return close();
    list.innerHTML = shown.map((o, i) => `<li role="option" data-i="${i}">${o.replace(/[&<>"]/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" })[c])}</li>`).join("");
    list.hidden = false;
    input.setAttribute("aria-expanded", "true");
    at = -1;
  };
  const choose = (value) => {
    input.value = value;
    close();
    input.dispatchEvent(new Event("input", { bubbles: true })); // the filters reload
  };
  input.addEventListener("input", open);
  input.addEventListener("focus", open);
  input.addEventListener("blur", () => setTimeout(close, 120));
  input.addEventListener("keydown", (e) => {
    if (list.hidden) return;
    if (e.key === "ArrowDown") {
      at = Math.min(at + 1, shown.length - 1);
      paint();
      e.preventDefault();
    } else if (e.key === "ArrowUp") {
      at = Math.max(at - 1, 0);
      paint();
      e.preventDefault();
    } else if (e.key === "Enter" && at >= 0) {
      choose(shown[at]);
      e.preventDefault();
    } else if (e.key === "Escape") close();
  });
  list.addEventListener("mousedown", (e) => {
    const li = e.target.closest("li");
    if (li) {
      e.preventDefault(); // keep focus in the box
      choose(shown[Number(li.dataset.i)]);
    }
  });
}
