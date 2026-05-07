// Cross-language parity test for the seeded roller.
//
// tests/fixtures/seeded-recipes.json is the contract: each case asserts that
// rollSeeded(recipe, vocab, seed, options) produces expected_name. Go, Python,
// and JS all run this fixture and must produce identical names.

import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import { dirname, resolve } from "node:path";

import { seededIndex } from "../src/seeded.js";
import { loadVocabularyFiles } from "../src/vocabulary.js";
import { loadRecipeBook } from "../src/recipe.js";

const here = dirname(fileURLToPath(import.meta.url));
const REPO_ROOT = resolve(here, "../..");
const FIXTURE_PATH = resolve(REPO_ROOT, "tests/fixtures/seeded-recipes.json");

async function loadFromRepo() {
  const vocab = await loadVocabularyFiles([
    resolve(REPO_ROOT, "vocabularies/realms.yaml"),
    resolve(REPO_ROOT, "vocabularies/adjectives.yaml"),
    resolve(REPO_ROOT, "vocabularies/nouns.yaml"),
    resolve(REPO_ROOT, "vocabularies/scientists.yaml"),
    resolve(REPO_ROOT, "vocabularies/creatures.yaml"),
  ]);
  const book = await loadRecipeBook(resolve(REPO_ROOT, "vocabularies/recipes.yaml"));
  return { vocab, book };
}

test("rollSeeded matches the cross-language fixture for every case", async () => {
  const { vocab, book } = await loadFromRepo();
  const fixture = JSON.parse(await readFile(FIXTURE_PATH, "utf8"));
  assert.ok(Array.isArray(fixture.cases), "fixture.cases must be an array");
  assert.ok(fixture.cases.length > 0, "fixture must have cases");

  for (const c of fixture.cases) {
    const got = await book.rollSeeded(c.recipe, vocab, c.seed, c.options || {});
    assert.equal(
      got,
      c.expected_name,
      `recipe=${c.recipe} seed=${JSON.stringify(c.seed)} options=${JSON.stringify(c.options)} got=${got} want=${c.expected_name}`,
    );
  }
});

test("seededIndex algorithm: SHA-256 BE first 8 bytes mod modulus", async () => {
  // Hand-checked vectors from the Go reference impl.
  // SHA-256("abc123" || BE_uint64(0)) first-8-BE mod 4 should be 1.
  assert.equal(await seededIndex("abc123", 0, 4), 1);
  assert.equal(await seededIndex("abc123", 1, 4), 2);
  assert.equal(await seededIndex("realmwatch", 0, 5), 0);
});

test("seededIndex returns 0 for non-positive modulus", async () => {
  assert.equal(await seededIndex("anything", 0, 0), 0);
  assert.equal(await seededIndex("anything", 5, -1), 0);
});

test("seededIndex output stays in [0, modulus)", async () => {
  for (let slot = 0; slot < 32; slot++) {
    const v = await seededIndex("test", slot, 7);
    assert.ok(v >= 0 && v < 7, `index ${v} out of range for modulus 7`);
  }
});

test("seededIndex is deterministic across calls", async () => {
  const a = await seededIndex("repeat", 5, 100);
  const b = await seededIndex("repeat", 5, 100);
  assert.equal(a, b);
});

test("seededIndex differs by slot", async () => {
  const a = await seededIndex("seed", 0, 100);
  const b = await seededIndex("seed", 1, 100);
  // Different slots can collide by chance, but for these small moduli they shouldn't.
  assert.notEqual(a, b);
});

test("seededIndex handles UTF-8 seeds", async () => {
  // 中文种子 should hash via UTF-8 bytes, matching Go and Python.
  const v = await seededIndex("中文种子", 0, 100);
  assert.ok(typeof v === "number" && v >= 0 && v < 100);
});

test("seededIndex handles empty seed", async () => {
  const v = await seededIndex("", 0, 100);
  assert.ok(typeof v === "number" && v >= 0 && v < 100);
});

test("seededIndex handles BigInt slot", async () => {
  const a = await seededIndex("seed", 0, 100);
  const b = await seededIndex("seed", 0n, 100);
  assert.equal(a, b);
});
