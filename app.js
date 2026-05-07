// app.js — landing page roller + catalog filter.
//
// Data is injected by build.sh into globals on window:
//   __LEXICON_VOCAB__    : { adjectives: {group: [words]}, nouns: {...}, ... }
//   __LEXICON_RECIPES__  : { project: {pattern, sources, required_options}, ... }
//   __LEXICON_REALMS__   : { fantasy: {description, words}, ... }
//
// The roller mirrors the cross-language seeded RNG from go/seeded.go and
// js/src/seeded.js (SHA-256 of utf8(seed) || be64(slot), first 8 bytes mod
// modulus) but generates a fresh random seed per roll since the landing page
// demo is not trying to be reproducible.

(() => {
  "use strict";

  const VOCAB   = window.__LEXICON_VOCAB__   || {};
  const RECIPES = window.__LEXICON_RECIPES__ || {};
  const REALMS  = window.__LEXICON_REALMS__  || {};

  const recipeSel  = document.getElementById("recipe");
  const realmField = document.getElementById("realm-field");
  const realmSel   = document.getElementById("realm");
  const prefixField = document.getElementById("prefix-field");
  const prefixIn    = document.getElementById("prefix");
  const nIn       = document.getElementById("n");
  const rollBtn   = document.getElementById("roll-btn");
  const candList  = document.getElementById("candidates");
  const meta      = document.getElementById("roll-meta");

  // Catalog
  const realmFilter = document.getElementById("catalog-realm");
  const catTable    = document.getElementById("catalog-table");
  const catCount    = document.getElementById("catalog-count");

  // ── populate recipe and realm selects ────────────────────────

  for (const [name, def] of Object.entries(RECIPES)) {
    const opt = document.createElement("option");
    opt.value = name;
    opt.textContent = `${name}  —  ${def.description || ""}`.trim();
    recipeSel.appendChild(opt);
  }

  for (const [name, def] of Object.entries(REALMS)) {
    const opt = document.createElement("option");
    opt.value = name;
    opt.textContent = `${name}  —  ${def.description || ""}`;
    realmSel.appendChild(opt);
  }

  // ── show / hide fields based on the current recipe ───────────

  function refreshFields() {
    const recipe = RECIPES[recipeSel.value];
    if (!recipe) return;
    const required = recipe.required_options || [];
    realmField.classList.toggle("hidden",  !required.includes("realm"));
    prefixField.classList.toggle("hidden", !required.includes("prefix"));
  }

  recipeSel.addEventListener("change", refreshFields);
  if (recipeSel.options.length > 0) {
    recipeSel.value = "project" in RECIPES ? "project" : recipeSel.options[0].value;
    refreshFields();
  }

  // ── seeded RNG (mirror of cross-language algorithm) ─────────

  function utf8Bytes(s) { return new TextEncoder().encode(s); }

  function be8(slot) {
    const out = new Uint8Array(8);
    let v = BigInt(slot);
    for (let i = 7; i >= 0; i--) {
      out[i] = Number(v & 0xffn);
      v >>= 8n;
    }
    return out;
  }

  async function seededIndex(seed, slot, modulus) {
    if (modulus <= 0) return 0;
    const seedBytes = utf8Bytes(seed);
    const slotBytes = be8(slot);
    const buf = new Uint8Array(seedBytes.length + 8);
    buf.set(seedBytes, 0);
    buf.set(slotBytes, seedBytes.length);
    const digest = new Uint8Array(await crypto.subtle.digest("SHA-256", buf));
    let v = 0n;
    for (let i = 0; i < 8; i++) v = (v << 8n) | BigInt(digest[i]);
    return Number(v % BigInt(modulus));
  }

  function randomSeed() {
    const bytes = new Uint8Array(16);
    crypto.getRandomValues(bytes);
    return Array.from(bytes, b => b.toString(16).padStart(2, "0")).join("");
  }

  // ── recipe rendering (simple {token:transform} interpolation) ─

  const TRANSFORMS = {
    cap:   s => s.length === 0 ? s : s[0].toUpperCase() + s.slice(1),
    lower: s => s.toLowerCase(),
    upper: s => s.toUpperCase(),
    raw:   s => s,
  };

  function parseTokens(pattern) {
    const re = /\{([^{}:]+)(?::([^{}]+))?\}/g;
    const tokens = [];
    let m;
    while ((m = re.exec(pattern)) !== null) {
      tokens.push({ name: m[1], transform: m[2] || "raw", index: m.index, length: m[0].length });
    }
    return tokens;
  }

  function applyTokens(pattern, tokens, values) {
    let out = pattern;
    for (let i = tokens.length - 1; i >= 0; i--) {
      const t = tokens[i];
      out = out.slice(0, t.index) + values[i] + out.slice(t.index + t.length);
    }
    return out;
  }

  function getWords(source, options) {
    let group = source.group;
    if (group === "fantasy" && options.realm) {
      // recipe declares fantasy by default, but caller chose a different realm — honor it
      group = options.realm;
    }
    const corpus = VOCAB[source.from] || {};
    const words = corpus[group];
    if (Array.isArray(words) && words.length > 0) return words;
    return corpus.any || [];
  }

  async function rollOne(recipeName, options, seed, candidateIdx) {
    const recipe = RECIPES[recipeName];
    if (!recipe) throw new Error(`unknown recipe: ${recipeName}`);
    const tokens = parseTokens(recipe.pattern);
    const values = [];
    for (let i = 0; i < tokens.length; i++) {
      const t = tokens[i];
      let value;
      if (t.name === "prefix") {
        value = options.prefix || "feat";
      } else {
        const src = (recipe.sources || {})[t.name];
        if (!src) throw new Error(`recipe ${recipeName} missing source for ${t.name}`);
        const words = getWords(src, options);
        if (words.length === 0) throw new Error(`empty corpus for ${t.name} (${src.from}/${src.group})`);
        const slot = candidateIdx * 1000 + i;
        const idx = await seededIndex(seed, slot, words.length);
        value = words[idx];
      }
      const xform = TRANSFORMS[t.transform] || TRANSFORMS.raw;
      values.push(xform(value));
    }
    return applyTokens(recipe.pattern, tokens, values);
  }

  async function rollN(recipeName, n, options) {
    const seed = randomSeed();
    const out = [];
    const seen = new Set();
    let safety = 0;
    let i = 0;
    while (out.length < n && safety < n * 8) {
      const name = await rollOne(recipeName, options, seed, i);
      if (!seen.has(name)) {
        seen.add(name);
        out.push(name);
      }
      i++;
      safety++;
    }
    return { seed, names: out };
  }

  rollBtn.addEventListener("click", async () => {
    candList.innerHTML = "";
    meta.textContent = "";
    rollBtn.disabled = true;
    try {
      const recipeName = recipeSel.value;
      const recipe = RECIPES[recipeName];
      const required = recipe.required_options || [];
      const options = {};
      if (required.includes("realm")) options.realm = realmSel.value;
      if (required.includes("prefix")) {
        const p = prefixIn.value.trim();
        if (!p) throw new Error("recipe needs a prefix (e.g., feat, fix)");
        options.prefix = p;
      }
      const n = Math.max(1, Math.min(20, parseInt(nIn.value, 10) || 5));
      const { seed, names } = await rollN(recipeName, n, options);
      for (const nm of names) {
        const div = document.createElement("div");
        div.className = "cand";
        div.textContent = nm;
        candList.appendChild(div);
      }
      meta.textContent = `seed ${seed.slice(0, 12)}…  ·  ${names.length} candidate${names.length === 1 ? "" : "s"}`;
    } catch (e) {
      const div = document.createElement("div");
      div.className = "err";
      div.textContent = String(e.message || e);
      candList.appendChild(div);
    } finally {
      rollBtn.disabled = false;
    }
  });

  // ── catalog filter ───────────────────────────────────────────

  if (realmFilter && catTable) {
    const rowRealms = new Set();
    catTable.querySelectorAll("tbody tr[data-realm]").forEach(tr => {
      rowRealms.add(tr.dataset.realm);
    });
    Array.from(rowRealms).sort().forEach(r => {
      const opt = document.createElement("option");
      opt.value = r;
      opt.textContent = r;
      realmFilter.appendChild(opt);
    });

    function applyFilter() {
      const want = realmFilter.value;
      let shown = 0, total = 0;
      catTable.querySelectorAll("tbody tr[data-realm]").forEach(tr => {
        total++;
        const match = !want || tr.dataset.realm === want;
        tr.classList.toggle("hidden", !match);
        if (match) shown++;
      });
      if (catCount) {
        catCount.textContent = (shown === total)
          ? `${total} project${total === 1 ? "" : "s"}`
          : `${shown} of ${total}`;
      }
    }

    realmFilter.addEventListener("change", applyFilter);
    applyFilter();
  }
})();
