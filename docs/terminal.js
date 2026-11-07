// The terminal on the page: screens of gist itself, written by demo/screens,
// drawn on a canvas. Each key leads to the screen it led to when gist ran.
(() => {
  const canvas = document.getElementById("terminal");
  if (!canvas || !matchMedia("(min-width: 900px)").matches) return;
  const note = document.getElementById("terminal-note");
  const ctx = canvas.getContext("2d");
  // Departure Mono is sharp at 11px, on cells of whole pixels. It has no bold.
  const FACE = '11px "Departure Mono", ui-monospace, Menlo, monospace';
  const CELL_W = 7;
  const CELL_H = 15;
  const BASELINE = 11;
  const ARROWS = { ArrowDown: "j", ArrowUp: "k", ArrowLeft: "h", ArrowRight: "l" };
  const ELSEWHERE = new Set(["enter", "n", "D", "E", "r", "y", "Y", "/", "c", "m", "o", "R"]);
  const PALETTE = ["#1e1e2e", "#f5a3b5", "#b5f0b0", "#ffe5a0", "#96bfff", "#d4b0ff", "#94e2d5", "#e0e4f0"];
  const FOREGROUND = "#d0d6e8";
  // Lines Redaction has no glyphs for: [up, right, down, left].
  const BOX = {
    "─": [0, 1, 0, 1], "│": [1, 0, 1, 0], "┌": [0, 1, 1, 0], "┐": [0, 0, 1, 1], "└": [1, 1, 0, 0], "┘": [1, 0, 0, 1],
    "╭": [0, 1, 1, 0], "╮": [0, 0, 1, 1], "╰": [1, 1, 0, 0], "╯": [1, 0, 0, 1], "├": [1, 1, 1, 0], "┤": [1, 0, 1, 1],
    "┬": [0, 1, 1, 1], "┴": [1, 1, 0, 1], "┼": [1, 1, 1, 1],
  };
  let data, at = 0, dpr;

  function styled(line) {
    const cells = [];
    let st = {};
    for (const part of line.split(/(\x1b\[[0-9;:]*m)/)) {
      const sgr = part.match(/^\x1b\[([0-9;:]*)m$/);
      if (!sgr) {
        for (const ch of part) cells.push({ ch, ...st });
        continue;
      }
      const a = (sgr[1] || "0").split(/[;:]/).map(Number);
      for (let i = 0; i < a.length; i++) {
        const n = a[i];
        if (n === 0) st = {};
        else if (n === 2) st.dim = true;
        else if (n === 22) st.dim = false;
        else if (n === 39) st.fg = null;
        else if (n === 49) st.bg = null;
        else if (n >= 30 && n <= 37) st.fg = PALETTE[n - 30];
        else if (n >= 40 && n <= 47) st.bg = PALETTE[n - 40];
        else if ((n === 38 || n === 48) && a[i + 1] === 2) {
          const c = `rgb(${a[i + 2]},${a[i + 3]},${a[i + 4]})`;
          if (n === 38) st.fg = c;
          else st.bg = c;
          i += 4;
        }
      }
    }
    return cells;
  }

  function line(x, y, c) {
    const [up, right, down, left] = BOX[c];
    const midX = x + Math.floor(CELL_W / 2);
    const midY = y + Math.floor(CELL_H / 2);
    if (left) ctx.fillRect(x, midY, midX - x + 1, 1);
    if (right) ctx.fillRect(midX, midY, x + CELL_W - midX, 1);
    if (up) ctx.fillRect(midX, y, 1, midY - y + 1);
    if (down) ctx.fillRect(midX, midY, 1, y + CELL_H - midY);
  }

  function draw() {
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    ctx.fillStyle = "#10111c";
    ctx.fillRect(0, 0, data.width * CELL_W, data.height * CELL_H);
    ctx.textAlign = "left";
    ctx.textBaseline = "alphabetic";
    ctx.font = FACE;
    data.screens[at].split("\n").forEach((text, row) => {
      styled(text).forEach((c, col) => {
        const x = col * CELL_W;
        const y = row * CELL_H;
        if (c.bg) {
          ctx.fillStyle = c.bg;
          ctx.fillRect(x, y, Math.ceil(CELL_W), Math.ceil(CELL_H));
        }
        if (c.ch === " ") return;
        ctx.fillStyle = c.fg || FOREGROUND;
        ctx.globalAlpha = c.dim ? 0.6 : 1;
        if (BOX[c.ch]) line(Math.round(x), Math.round(y), c.ch);
        else if (c.ch === "┃") ctx.fillRect(Math.round(x + CELL_W / 2) - 1, y, 2, Math.ceil(CELL_H));
        else if (c.ch === "▎") ctx.fillRect(x, y, 2, Math.ceil(CELL_H));
        else ctx.fillText(c.ch, x, y + BASELINE);
        ctx.globalAlpha = 1;
      });
    });
  }

  function fit() {
    dpr = devicePixelRatio || 1;
    canvas.width = data.width * CELL_W * dpr;
    canvas.height = data.height * CELL_H * dpr;
    canvas.style.width = `${data.width * CELL_W}px`;
    canvas.style.height = `${data.height * CELL_H}px`;
    draw();
  }

  function press(key) {
    const to = data.next[at][key];
    if (to === undefined) return false;
    at = to;
    draw();
    return true;
  }

  canvas.addEventListener("keydown", (e) => {
    if (e.metaKey || e.ctrlKey || e.altKey) return;
    const key = ARROWS[e.key] || (e.key === "Enter" ? "enter" : e.key);
    if (key === "Escape") {
      if (data.help[at]) press("?");
      return;
    }
    if (press(key)) {
      note.textContent = "";
      e.preventDefault();
    } else if (ELSEWHERE.has(key)) {
      note.textContent = "That one needs your own terminal: this page only shows gist.";
      e.preventDefault();
    }
  });

  Promise.all([
    fetch("screens.json").then((r) => r.json()),
    document.fonts.load(FACE),
  ]).then(([screens]) => {
    data = screens;
    document.documentElement.classList.add("live");
    fit();
    const keys = new URLSearchParams(location.search).get("keys");
    for (const k of keys || "") press(k);
  });
})();
