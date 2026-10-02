(() => {
  const root = document.documentElement;

  // Theme: remember the reader's choice.
  document.querySelector(".theme")?.addEventListener("click", () => {
    const next = root.dataset.theme === "light" ? "dark" : "light";
    root.dataset.theme = next;
    try { localStorage.setItem("theme", next); } catch (e) {}
  });

  // Navigation on small screens.
  const menu = document.querySelector(".menu");
  menu?.addEventListener("click", () => {
    const open = document.body.classList.toggle("nav-open");
    menu.setAttribute("aria-expanded", String(open));
  });

  // Copy buttons.
  document.querySelectorAll(".copy").forEach((button) => {
    button.addEventListener("click", async () => {
      const code = button.parentElement.querySelector("code")?.innerText ?? "";
      try {
        await navigator.clipboard.writeText(code);
        button.textContent = "Copied";
      } catch (e) {
        button.textContent = "Select and copy";
      }
      setTimeout(() => { button.textContent = "Copy"; }, 1500);
    });
  });

  // Highlight the current section in "On this page".
  const tocLinks = [...document.querySelectorAll(".toc a")];
  if (tocLinks.length && "IntersectionObserver" in window) {
    const byId = new Map(tocLinks.map((a) => [a.getAttribute("href").slice(1), a]));
    const observer = new IntersectionObserver((entries) => {
      for (const entry of entries) {
        if (entry.isIntersecting) {
          tocLinks.forEach((a) => a.classList.remove("active"));
          byId.get(entry.target.id)?.classList.add("active");
        }
      }
    }, { rootMargin: "-70px 0px -70% 0px" });
    document.querySelectorAll("h2[id], h3[id]").forEach((h) => observer.observe(h));
  }

  // The greenhouse sparks, ported from the original Vein page: decorative,
  // capped at 30fps, paused off screen, in hidden tabs and for reduced motion.
  const scene = document.querySelector(".hero-art .scene");
  const canvas = scene?.querySelector("canvas");
  const svg = scene?.querySelector("svg");
  const ctx = canvas?.getContext("2d");
  if (scene && canvas && svg && ctx) {
    const width = 1078, height = 309, samples = 240;
    ctx.imageSmoothingEnabled = false;
    const routes = [...svg.querySelectorAll("path")].map((path) => {
      const length = path.getTotalLength();
      return Array.from({ length: samples + 1 }, (_, i) => path.getPointAtLength(length * i / samples));
    });
    const motion = matchMedia("(prefers-reduced-motion: reduce)");
    let reduced = motion.matches, frame = 0, previous = 0, clock = 0;
    let inView = typeof IntersectionObserver === "undefined";
    let seed = 913;
    const random = () => { seed = (Math.imul(seed, 1664525) + 1013904223) >>> 0; return seed / 4294967296; };
    const flies = Array.from({ length: 42 }, () => ({ x: random() * width, y: 8 + random() * 250, phase: random() * 6.28, speed: .35 + random() * .55 }));
    const reflections = Array.from({ length: 28 }, () => ({ x: random() * width, y: 270 + random() * 32, width: 2 + random() * 11, phase: random() * 6.28 }));
    const pixel = (x, y, size, alpha, glow = 0) => {
      ctx.fillStyle = `rgba(98,255,153,${alpha})`;
      ctx.shadowColor = "#62ff99";
      ctx.shadowBlur = glow;
      ctx.fillRect(Math.round(x), Math.round(y), size, size);
      ctx.shadowBlur = 0;
    };
    const draw = () => {
      ctx.clearRect(0, 0, width, height);
      for (const fly of flies) {
        const phase = clock * fly.speed + fly.phase;
        pixel(fly.x + Math.sin(phase) * 8, fly.y + Math.cos(phase * .6) * 5, 1, .12 + ((Math.sin(phase) + 1) / 2) ** 3 * .5, 2);
      }
      routes.forEach((points, index) => {
        for (let spark = 0; spark < 3; spark++) {
          const progress = (clock * .18 + index * .137 + spark / 3) % 1;
          for (let tail = 5; tail >= 0; tail--) {
            const position = ((progress - tail * .008 + 1) % 1) * samples;
            const i = Math.floor(position);
            const a = points[i], b = points[Math.min(i + 1, samples)];
            if (!a || !b) continue;
            const mix = position - i;
            pixel(a.x + (b.x - a.x) * mix, a.y + (b.y - a.y) * mix, tail === 0 ? 2 : 1, (1 - tail / 6) * .95, tail === 0 ? 7 : 0);
          }
        }
      });
      for (const r of reflections) {
        ctx.fillStyle = `rgba(98,255,153,${.04 + (Math.sin(clock * 1.7 + r.phase) + 1) * .07})`;
        ctx.fillRect(Math.round(r.x), Math.round(r.y), r.width, 1);
      }
    };
    const tick = (now) => {
      frame = 0;
      if (document.hidden || !inView || reduced) return;
      if (!previous || now - previous >= 1000 / 30) {
        clock += previous ? Math.min((now - previous) / 1000, .1) : 0;
        previous = now;
        draw();
      }
      frame = requestAnimationFrame(tick);
    };
    const sync = () => {
      if (frame) cancelAnimationFrame(frame);
      frame = 0;
      previous = 0;
      const running = inView && !document.hidden && !reduced;
      scene.dataset.animated = String(running);
      if (reduced) ctx.clearRect(0, 0, width, height);
      else if (running) { draw(); frame = requestAnimationFrame(tick); }
    };
    if (typeof IntersectionObserver !== "undefined") {
      new IntersectionObserver((entries) => { inView = entries.some((e) => e.isIntersecting); sync(); }, { threshold: 0 }).observe(scene);
    }
    document.addEventListener("visibilitychange", sync);
    motion.addEventListener("change", (e) => { reduced = e.matches; sync(); });
    sync();
  }

  // Search: a small index of titles, headings and text, loaded on first use.
  const input = document.querySelector(".search input");
  const box = document.querySelector(".results");
  if (!input || !box) return;
  let index = null;
  let selected = -1;

  const load = async () => {
    if (index) return index;
    const response = await fetch("/search.json");
    index = await response.json();
    return index;
  };
  const escape = (s) => s.replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));

  const search = (query) => {
    const words = query.toLowerCase().split(/\s+/).filter(Boolean);
    const hits = [];
    for (const page of index) {
      const title = page.t.toLowerCase();
      const text = page.x.toLowerCase();
      const all = (title + " " + page.d + " " + text).toLowerCase();
      if (!words.every((w) => all.includes(w))) continue;
      let score = words.reduce((n, w) => n + (title.includes(w) ? 10 : 0) + (page.d.toLowerCase().includes(w) ? 3 : 0), 0);
      hits.push({ score, title: page.t, url: page.u, note: page.d });
      for (const h of page.h || []) {
        const ht = h.t.toLowerCase();
        if (words.every((w) => ht.includes(w))) {
          hits.push({ score: score + 6 + words.length, title: h.t, url: page.u + "#" + h.a, note: page.t });
        }
      }
    }
    return hits.sort((a, b) => b.score - a.score).slice(0, 10);
  };

  const show = async () => {
    const query = input.value.trim();
    if (!query) { box.hidden = true; return; }
    await load();
    const hits = search(query);
    selected = hits.length ? 0 : -1;
    box.innerHTML = hits.length
      ? hits.map((h, i) => `<a href="${h.url}" role="option" aria-selected="${i === 0}">${escape(h.title)}<span>${escape(h.note)}</span></a>`).join("")
      : `<p>No results for “${escape(query)}”.</p>`;
    box.hidden = false;
  };
  const move = (delta) => {
    const items = [...box.querySelectorAll("a")];
    if (!items.length) return;
    selected = (selected + delta + items.length) % items.length;
    items.forEach((a, i) => a.setAttribute("aria-selected", String(i === selected)));
    items[selected].scrollIntoView({ block: "nearest" });
  };

  input.addEventListener("input", show);
  input.addEventListener("focus", () => { load(); if (input.value) show(); });
  input.addEventListener("keydown", (e) => {
    if (e.key === "ArrowDown") { e.preventDefault(); move(1); }
    else if (e.key === "ArrowUp") { e.preventDefault(); move(-1); }
    else if (e.key === "Enter") {
      const target = box.querySelectorAll("a")[selected];
      if (target) location.href = target.getAttribute("href");
    } else if (e.key === "Escape") { box.hidden = true; input.blur(); }
  });
  document.addEventListener("click", (e) => { if (!e.target.closest(".search")) box.hidden = true; });
  document.addEventListener("keydown", (e) => {
    if ((e.key === "/" || (e.key === "k" && (e.metaKey || e.ctrlKey))) && document.activeElement !== input) {
      e.preventDefault();
      input.focus();
    }
  });
})();
