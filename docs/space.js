// The page in two tones: a moon, a banded planet and a probe, ordered
// dithered onto a coarse grid of pixels. Every pixel is either ink or paper.
(() => {
  const canvas = document.getElementById("field");
  const still = matchMedia("(prefers-reduced-motion: reduce)");
  const PIXEL = 2;
  const FRAME = 1000 / 12;
  const INK = 0xff0c0605;
  const PAPER = 0xfff2e8e6;
  const BAYER = [0, 8, 2, 10, 12, 4, 14, 6, 3, 11, 1, 9, 15, 7, 13, 5].map((v) => (v + 0.5) / 16);
  const TEX_W = 2048;
  const TEX_H = 1024;
  const ctx = canvas.getContext("2d");
  let W, H, image, pixels, out, depth, dust, twinkles, moon, ball, sparks;
  let texture;

  const random = (i, j) => {
    const s = Math.sin(i * 12.9898 + j * 78.233) * 43758.5453;
    return s - Math.floor(s);
  };
  const noise = (x, y) => {
    const xi = Math.floor(x);
    const yi = Math.floor(y);
    const u = x - xi;
    const v = y - yi;
    const a = random(xi, yi) * (1 - u) + random(xi + 1, yi) * u;
    const b = random(xi, yi + 1) * (1 - u) + random(xi + 1, yi + 1) * u;
    return a * (1 - v) + b * v;
  };
  const fbm = (x, y) => noise(x, y) * 0.5 + noise(x * 2.3, y * 2.3) * 0.3 + noise(x * 5.1, y * 5.1) * 0.2;
  const dither = (x, y) => BAYER[(x & 3) + (y & 3) * 4];

  // The surface: craters as heights, lit from one side once they are all in.
  function makeTexture() {
    const height = new Float32Array(TEX_W * TEX_H);
    for (let i = 0; i < 520; i++) {
      const cu = Math.floor(random(i, 1) * TEX_W);
      const cv = Math.floor(random(i, 2) * TEX_H);
      const r = 4 + Math.pow(random(i, 3), 3) * 72;
      const reach = Math.ceil(r * 1.5);
      for (let v = Math.max(0, cv - reach); v < Math.min(TEX_H, cv + reach); v++) {
        for (let u = cu - reach; u < cu + reach; u++) {
          const d = Math.hypot(u - cu, v - cv) / r;
          if (d > 1.5) continue;
          const bowl = d < 1 ? (d * d - 1) * r * 0.42 : 0;
          const rim = Math.exp(-((d - 1) ** 2) / 0.03) * r * 0.16;
          height[((u + TEX_W) % TEX_W) + v * TEX_W] += bowl + rim;
        }
      }
    }
    texture = new Float32Array(TEX_W * TEX_H);
    for (let v = 1; v < TEX_H - 1; v++) {
      for (let u = 1; u < TEX_W - 1; u++) {
        const i = u + v * TEX_W;
        const slope = (height[i - 1] - height[i + 1]) * 0.55 + (height[i - TEX_W] - height[i + TEX_W]) * 0.75;
        texture[i] = slope * 0.2 + (fbm(u * 0.03, v * 0.03) - 0.5) * 0.34;
      }
    }
  }

  // flat lays the surface out so that a round crater stays round on it, with
  // the part of the moon that faces up to the sky at the middle.
  function flat(nx, ny, nz) {
    const y = ny * 0.69 + nz * 0.72;
    const z = -ny * 0.72 + nz * 0.69;
    return { sx: nx / (1 + z), sy: y / (1 + z) };
  }

  function makeSphere(cx, cy, r, light) {
    const list = [];
    for (let y = Math.max(0, Math.floor(cy - r)); y < Math.min(H, Math.ceil(cy + r)); y++) {
      for (let x = Math.max(0, Math.floor(cx - r)); x < Math.min(W, Math.ceil(cx + r)); x++) {
        const nx = (x - cx) / r;
        const ny = (y - cy) / r;
        const d = nx * nx + ny * ny;
        if (d >= 1) continue;
        const nz = Math.sqrt(1 - d);
        list.push({
          i: x + y * W,
          x,
          y,
          lit: Math.max(0, nx * light[0] + ny * light[1] + nz * light[2]),
          lon: Math.atan2(nx, nz),
          lat: Math.asin(ny),
          edge: nz,
          ...flat(nx, ny, nz),
        });
      }
    }
    return list;
  }

  function resize() {
    W = Math.ceil(innerWidth / PIXEL);
    H = Math.ceil(innerHeight / PIXEL);
    canvas.width = W;
    canvas.height = H;
    canvas.style.imageRendering = "pixelated";
    image = ctx.createImageData(W, H);
    pixels = new Uint32Array(image.data.buffer);
    out = new Uint8Array(W * H);
    depth = new Float32Array(W * H);
    dust = new Uint8Array(W * H);
    twinkles = [];
    for (let y = 0; y < H; y++) {
      for (let x = 0; x < W; x++) {
        const band = Math.pow(fbm((x / W) * 3 + 4, (y / H) * 2.2 + 1), 4) * 0.34;
        if (random(x, y) < 0.006 + band) dust[x + y * W] = 1;
        if (random(y, x) < 0.0009) twinkles.push({ i: x + y * W, phase: random(x, 7) * 6.28 });
      }
    }
    const size = Math.min(W, H);
    const R = Math.max(W * 0.95, H * 0.95);
    moon = makeSphere(W * 0.46, H + R * 0.74, R, [-0.42, -0.72, 0.55]);
    const wide = W > H;
    ball = makeSphere(W * (wide ? 0.86 : 0.8), H * (wide ? 0.19 : 0.13), size * 0.085, [-0.72, -0.2, 0.66]);
    sparks = [
      [0.49, 0.2, 13],
      [0.38, 0.62, 10],
      [0.94, 0.5, 8],
      [0.7, 0.08, 7],
      [0.08, 0.66, 11],
      [0.9, 0.7, 12],
    ].map(([x, y, r]) => ({ x: Math.round(x * W), y: Math.round(y * H), r }));
  }

  function spark(s, t, n) {
    const r = s.r * (0.75 + 0.25 * Math.sin(t * 0.8 + n * 2));
    for (let k = -Math.ceil(r); k <= r; k++) {
      const fade = 1 - Math.abs(k) / r;
      if (fade < dither(k + 8, n) * 0.6) continue;
      set(s.x + k, s.y);
      set(s.x, s.y + k);
    }
    set(s.x + 1, s.y + 1);
    set(s.x - 1, s.y - 1);
    set(s.x + 1, s.y - 1);
    set(s.x - 1, s.y + 1);
  }

  function set(x, y) {
    if (x >= 0 && y >= 0 && x < W && y < H) out[x + y * W] = 1;
  }

  // A point of the probe, hidden behind what is nearer.
  function plot(p, level) {
    const x = Math.round(p[0]);
    const y = Math.round(p[1]);
    if (x < 0 || y < 0 || x >= W || y >= H) return;
    const i = x + y * W;
    if (p[2] < depth[i]) return;
    depth[i] = p[2];
    out[i] = level > dither(x, y) ? 1 : 0;
  }

  function probe(t) {
    const unit = Math.min(W, H) * 0.0036;
    const yaw = -0.62 + Math.sin(t * 0.07) * 0.2;
    const pitch = 0.34 + Math.sin(t * 0.05) * 0.1;
    const roll = 0.46 + Math.sin(t * 0.04) * 0.08;
    const cx = W * (W > H ? 0.68 : 0.56);
    const cy = H * (W > H ? 0.36 : 0.46) + Math.sin(t * 0.2) * 3;
    const [cy1, sy1, cp, sp, cr, sr] = [Math.cos(yaw), Math.sin(yaw), Math.cos(pitch), Math.sin(pitch), Math.cos(roll), Math.sin(roll)];
    const turn = ([x, y, z]) => {
      const x1 = x * cy1 + z * sy1;
      const z1 = -x * sy1 + z * cy1;
      const y1 = y * cp - z1 * sp;
      const z2 = y * sp + z1 * cp;
      return [x1 * cr - y1 * sr, x1 * sr + y1 * cr, z2];
    };
    const place = (v) => {
      const [x, y, z] = turn(v);
      return [cx + x * unit, cy + y * unit, z];
    };
    const light = [-0.55, -0.62, 0.56];
    const shade = (n) => {
      const [x, y, z] = turn(n);
      return x * light[0] + y * light[1] + z * light[2];
    };
    const line = (a, b, level = 1, gap = 0) => {
      const pa = place(a);
      const pb = place(b);
      const steps = Math.ceil(Math.hypot(pb[0] - pa[0], pb[1] - pa[1]) * 1.4) + 1;
      for (let n = 0; n <= steps; n++) {
        if (gap && n % gap === 0) continue;
        const m = n / steps;
        plot([pa[0] + (pb[0] - pa[0]) * m, pa[1] + (pb[1] - pa[1]) * m, pa[2] + (pb[2] - pa[2]) * m], level);
      }
    };
    // a lattice boom: rails with a zigzag between them
    const boom = (a, b, width, cells) => {
      const axis = [b[0] - a[0], b[1] - a[1], b[2] - a[2]];
      const side = [0, width, 0];
      const lift = [0, 0, width];
      const rail = (o) => line([a[0] + o[0], a[1] + o[1], a[2] + o[2]], [b[0] + o[0], b[1] + o[1], b[2] + o[2]]);
      rail(side);
      rail([0, -width, 0]);
      rail(lift);
      for (let n = 0; n < cells; n++) {
        const p = (m) => [a[0] + axis[0] * m, a[1] + axis[1] * m, a[2] + axis[2] * m];
        const [u, v] = [p(n / cells), p((n + 1) / cells)];
        line([u[0], u[1] + width, u[2]], [v[0], v[1] - width, v[2]]);
        line([u[0], u[1] - width, u[2]], [v[0], v[1], v[2] + width]);
      }
    };
    const cylinder = (x0, x1, r, tiltY) => {
      const step = 0.34 / Math.max(unit, 1);
      for (let x = x0; x <= x1; x += step * 2) {
        for (let a = 0; a < 6.28; a += step / r) {
          const n = [0, Math.cos(a), Math.sin(a)];
          plot(place([x, tiltY[0] + (x - x0) * tiltY[1] + n[1] * r, n[2] * r]), 0.12 + Math.max(0, shade(n)) * 0.95);
        }
      }
    };
    depth.fill(-1e9);
    // long wires first, behind all the rest
    line([-30, 6, 0], [-120, 96, 40], 1, 3);
    line([-10, -8, 6], [150, -86, -30], 1);
    // the bus and its frame
    cylinder(-30, -10, 10.5, [0, 0]);
    for (let a = 0; a < 6; a++) {
      const x = Math.cos(a * 1.047) * 11;
      const y = Math.sin(a * 1.047) * 11;
      line([-36, x * 0.45, y * 0.45], [-10, x, y]);
      line([-36, x * 0.45, y * 0.45], [-30, Math.cos(a * 1.047 + 1) * 11, Math.sin(a * 1.047 + 1) * 11]);
    }
    boom([-28, 8, 0], [-74, 56, 16], 2.2, 12);
    boom([-28, -8, 0], [-58, -62, -18], 2.6, 10);
    {
      const c = place([-74, 56, 16]);
      for (let a = 0; a < 6.28; a += 0.02) {
        for (let r = 0; r < 7 * unit; r += 0.7) plot([c[0] + Math.cos(a) * r, c[1] + Math.sin(a) * r * 0.85, c[2] + 2], 0.15 + (0.5 - Math.cos(a - 3.9) * 0.5) * (r / (7 * unit)));
      }
    }
    // the dish, a shallow bowl seen from behind
    const reach = 36;
    const dr = 0.5 / unit;
    for (let r = 0; r <= reach; r += dr) {
      const da = 0.5 / (unit * Math.max(r, 1));
      for (let a = 0; a < 6.283; a += da) {
        const y = Math.cos(a) * r;
        const z = Math.sin(a) * r;
        const x = (r * r) / 62 - 8;
        const n = [-1, (y / 31), (z / 31)];
        const len = Math.hypot(...n);
        const lit = shade([n[0] / len, n[1] / len, n[2] / len]);
        plot(place([x, y, z]), r > reach - 1.2 ? 1 : 0.06 + Math.max(0, lit) * 1.15);
      }
    }
  }

  function draw(t) {
    out.set(dust);
    for (const s of twinkles) out[s.i] = Math.sin(t * 1.6 + s.phase) > 0 ? 1 : 0;
    sparks.forEach((s, n) => spark(s, t, n));
    const turnBall = t * 0.05;
    for (const p of ball) {
      const swirl = Math.sin(p.lat * 12 + Math.sin(p.lon * 2 + turnBall) * 1.2 + Math.sin(p.lat * 31) * 0.5);
      const level = p.lit * (0.62 + swirl * 0.42) + (p.edge < 0.2 ? 0.1 : 0);
      out[p.i] = level > dither(p.x, p.y) ? 1 : 0;
    }
    const turnMoon = t * 0.0035;
    for (const p of moon) {
      const u = Math.floor((p.sx + turnMoon + 2) * 1500) % TEX_W;
      const v = Math.floor((p.sy + 2) * 1500) % TEX_H;
      const level = p.lit * 0.95 + texture[u + v * TEX_W] * (0.35 + p.lit);
      out[p.i] = level > dither(p.x, p.y) ? 1 : 0;
    }
    probe(t);
    clearAroundText();
    for (let i = 0; i < out.length; i++) pixels[i] = out[i] ? PAPER : INK;
    ctx.putImageData(image, 0, 0);
  }

  // The stars thin out toward the words, so that no box shows around them.
  function clearAroundText() {
    const margin = 26;
    for (const el of document.querySelectorAll("header h1, header p, header nav")) {
      const r = el.getBoundingClientRect();
      const range = document.createRange();
      range.selectNodeContents(el);
      const width = range.getBoundingClientRect().width;
      const left = r.left / PIXEL;
      const right = (r.left + width) / PIXEL;
      const top = r.top / PIXEL;
      const bottom = r.bottom / PIXEL;
      const fromY = Math.max(0, Math.floor(top - margin));
      const toY = Math.min(H, Math.ceil(bottom + margin));
      const fromX = Math.max(0, Math.floor(left - margin));
      const toX = Math.min(W, Math.ceil(right + margin));
      for (let y = fromY; y < toY; y++) {
        const dy = Math.max(top - y, y - bottom, 0);
        for (let x = fromX; x < toX; x++) {
          const i = x + y * W;
          if (!out[i]) continue;
          const dx = Math.max(left - x, x - right, 0);
          if (Math.hypot(dx, dy) / margin < random(x, y + 3)) out[i] = 0;
        }
      }
    }
  }

  let last = 0;
  function tick(now) {
    if (still.matches) return;
    requestAnimationFrame(tick);
    if (document.hidden || now - last < FRAME) return;
    last = now;
    draw(now / 1000);
  }

  function start() {
    resize();
    draw(40);
    if (!still.matches) requestAnimationFrame(tick);
  }

  const fade = () => {
    canvas.style.opacity = 1 - Math.min(1, scrollY / (innerHeight * 0.8)) * 0.4;
  };
  makeTexture();
  addEventListener("resize", () => {
    resize();
    draw(performance.now() / 1000);
  });
  addEventListener("scroll", fade, { passive: true });
  still.addEventListener("change", start);
  fade();
  document.fonts.load('700 200px "Redaction 70"').finally(start);
})();
