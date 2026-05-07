// Recipe engine. Mirrors go/recipe.go.
//
// Pattern tokens: {slot:transform}. Transforms: cap, lower, upper, raw.
// A slot name may appear multiple times; each occurrence is an independent
// roll, with uniqueness enforced within a single name.

import { readFile } from "node:fs/promises";
import yaml from "js-yaml";
import { seededIndex } from "./seeded.js";

const SLOT_RE = /\{([a-z_]+)(?::([a-z]+))?\}/g;

export class RecipeBook {
  constructor(recipes = {}) {
    this.recipes = recipes;
  }

  has(name) {
    return Object.prototype.hasOwnProperty.call(this.recipes, name);
  }

  names() {
    return Object.keys(this.recipes);
  }

  describe(name) {
    const r = this.recipes[name];
    return r ? r.description || "" : "";
  }

  requiredOptions(name) {
    const r = this.recipes[name];
    if (!r) return [];
    return r.required_options || [];
  }

  async roll(name, vocabulary, options = {}) {
    return this._fill(name, vocabulary, options, async (_slot, modulus) =>
      Math.floor(Math.random() * modulus),
    );
  }

  async rollSeeded(name, vocabulary, seed, options = {}) {
    return this._fill(name, vocabulary, options, (slot, modulus) =>
      seededIndex(seed, slot, modulus),
    );
  }

  async rollN(name, vocabulary, n, options = {}) {
    const out = [];
    const seen = new Set();
    let attempts = 0;
    const cap = Math.max(n * 10, 50);
    while (out.length < n && attempts < cap) {
      attempts++;
      const candidate = await this.roll(name, vocabulary, options);
      if (!seen.has(candidate)) {
        seen.add(candidate);
        out.push(candidate);
      }
    }
    return out;
  }

  async _fill(name, vocabulary, options, pickIndex) {
    const r = this.recipes[name];
    if (!r) {
      throw new Error(
        `unknown recipe ${JSON.stringify(name)} (have ${this.names().join(", ")})`,
      );
    }
    checkRequired(r, options);
    const tokens = parsePattern(r.pattern || "");
    const slotCounter = Object.create(null);
    const picked = Object.create(null);
    let out = "";
    for (const tk of tokens) {
      if (tk.literal !== undefined) {
        out += tk.literal;
        continue;
      }
      const optVal = resolveOption(tk.slot, options);
      if (optVal !== undefined) {
        out += applyTransform(optVal, tk.transform);
        continue;
      }
      const src = (r.sources || {})[tk.slot];
      if (!src) {
        throw new Error(
          `recipe ${JSON.stringify(r.description || name)}: slot ${JSON.stringify(tk.slot)} has no source and no option provided`,
        );
      }
      let group = src.group;
      if (group === "fantasy" && options.realm) {
        group = options.realm;
      }
      const words = vocabulary.group(src.from, group);
      if (!words || words.length === 0) {
        throw new Error(
          `recipe ${JSON.stringify(r.description || name)}: source ${src.from}.${group} missing or empty`,
        );
      }
      let seen = picked[tk.slot];
      if (!seen) {
        seen = new Set();
        picked[tk.slot] = seen;
      }
      let idx = 0;
      let local = slotCounter[tk.slot] || 0n;
      for (let attempt = 0; attempt < words.length; attempt++) {
        const candidate = await pickIndex(local, words.length);
        local = local + 1n;
        if (!seen.has(candidate) || seen.size >= words.length) {
          idx = candidate;
          break;
        }
      }
      slotCounter[tk.slot] = local;
      seen.add(idx);
      out += applyTransform(words[idx], tk.transform);
    }
    return out;
  }
}

export async function loadRecipeBook(path) {
  const text = await readFile(path, "utf8");
  const raw = yaml.load(text) || {};
  return new RecipeBook(raw.recipes || {});
}

function parsePattern(pat) {
  const tokens = [];
  let idx = 0;
  SLOT_RE.lastIndex = 0;
  let m;
  while ((m = SLOT_RE.exec(pat)) !== null) {
    if (m.index > idx) {
      tokens.push({ literal: pat.slice(idx, m.index) });
    }
    tokens.push({ slot: m[1], transform: m[2] || "raw" });
    idx = m.index + m[0].length;
  }
  if (idx < pat.length) {
    tokens.push({ literal: pat.slice(idx) });
  }
  return tokens;
}

function applyTransform(word, transform) {
  switch (transform) {
    case "cap":
      if (!word) return word;
      return word[0].toUpperCase() + word.slice(1);
    case "lower":
      return word.toLowerCase();
    case "upper":
      return word.toUpperCase();
    default:
      return word;
  }
}

function resolveOption(slot, opts) {
  if (slot === "prefix") {
    if (!opts.prefix) {
      throw new Error("option 'prefix' is required");
    }
    return opts.prefix;
  }
  if (slot === "realm") {
    if (!opts.realm) {
      throw new Error("option 'realm' is required");
    }
    return opts.realm;
  }
  return undefined;
}

function checkRequired(recipe, opts) {
  const req = recipe.required_options || [];
  for (const r of req) {
    if (r === "realm" && !opts.realm) {
      throw new Error("recipe requires --realm");
    }
    if (r === "prefix" && !opts.prefix) {
      throw new Error("recipe requires --prefix");
    }
  }
}

export const _internal = { parsePattern, applyTransform };
