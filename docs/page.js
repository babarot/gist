// The time since the first commit of gist, counted as a probe's would be.
(() => {
  const launch = Date.parse("2017-04-19T12:39:47+09:00");
  const out = document.getElementById("elapsed");
  const two = (n) => String(n).padStart(2, "0");
  const show = () => {
    const seconds = Math.floor((Date.now() - launch) / 1000);
    const days = Math.floor(seconds / 86400);
    const h = Math.floor(seconds / 3600) % 24;
    const m = Math.floor(seconds / 60) % 60;
    out.textContent = `${days}d ${two(h)}:${two(m)}:${two(seconds % 60)}`;
  };
  show();
  if (!matchMedia("(prefers-reduced-motion: reduce)").matches) setInterval(show, 1000);
})();
