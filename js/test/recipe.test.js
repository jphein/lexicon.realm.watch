// Recipe engine tests. Uses the small frozen test corpus at
// tests/fixtures/vocabularies-test.yaml so vocab edits don't break tests.

import test from "node:test";
import assert from "node:assert/strict";
import { fileURLToPath } from "node:url";
import { dirname, resolve } from "node:path";

import { Vocabulary, loadVocabularyFile } from "../src/vocabulary.js";
import { RecipeBook, loadRecipeBook } from "../src/recipe.js";

const here = dirname(fileURLToPath(import.meta.url));
const TEST_VOCAB = resolve(here, "../../tests/fixtures/vocabularies-test.yaml");
const REAL_RECIPES = resolve(here, "../../vocabularies/recipes.yaml");

function buildVocab() {
  const v = new Vocabulary();
  v.addGroup("adjectives", "any", ["quick", "brisk"]);
  v.addGroup("adjectives", "fantasy", ["primal", "shadow"]);
  v.addGroup("nouns", "any", ["forge", "beacon"]);
  v.addGroup("nouns", "fantasy", ["sigil", "rune"]);
  v.addGroup("scientists", "any", ["curie", "einstein"]);
  v.addGroup("creatures", "any", ["dragon", "kelpie"]);
  return v;
}

function buildBook() {
  return new RecipeBook({
    project: {
      description: "Project name",
      pattern: "{adjective:cap}-{noun:cap}",
      sources: {
        adjective: { from: "adjectives", group: "fantasy" },
        noun: { from: "nouns", group: "fantasy" },
      },
      required_options: ["realm"],
    },
    agent: {
      description: "Agent name",
      pattern: "{adjective:lower}_{scientist:lower}",
      sources: {
        adjective: { from: "adjectives", group: "any" },
        scientist: { from: "scientists", group: "any" },
      },
    },
    branch: {
      description: "Branch name",
      pattern: "{prefix}/{adjective:lower}-{noun:lower}-{noun:lower}",
      sources: {
        adjective: { from: "adjectives", group: "any" },
        noun: { from: "nouns", group: "any" },
      },
      required_options: ["prefix"],
    },
  });
}

test("RecipeBook.has and names()", () => {
  const book = buildBook();
  assert.ok(book.has("project"));
  assert.ok(book.has("agent"));
  assert.ok(!book.has("nonsense"));
  const names = book.names().sort();
  assert.deepEqual(names, ["agent", "branch", "project"]);
});

test("agent recipe rolls a snake_case name", async () => {
  const book = buildBook();
  const v = buildVocab();
  const name = await book.roll("agent", v);
  assert.match(name, /^[a-z]+_[a-z]+$/);
});

test("project recipe requires --realm", async () => {
  const book = buildBook();
  const v = buildVocab();
  await assert.rejects(book.roll("project", v, {}), /realm/);
});

test("branch recipe requires --prefix", async () => {
  const book = buildBook();
  const v = buildVocab();
  await assert.rejects(book.roll("branch", v, {}), /prefix/);
});

test("rollSeeded is deterministic across calls", async () => {
  const book = buildBook();
  const v = buildVocab();
  const a = await book.rollSeeded("agent", v, "my-seed");
  const b = await book.rollSeeded("agent", v, "my-seed");
  assert.equal(a, b);
});

test("rollSeeded with different seeds yields different names eventually", async () => {
  const book = buildBook();
  const v = buildVocab();
  const seeds = ["s1", "s2", "s3", "s4", "s5"];
  const results = new Set();
  for (const s of seeds) {
    results.add(await book.rollSeeded("agent", v, s));
  }
  // 5 seeds, small corpus — at least 2 distinct outcomes.
  assert.ok(results.size >= 2, `expected variability, got ${[...results].join(",")}`);
});

test("rollN returns up to N candidates", async () => {
  const book = buildBook();
  const v = buildVocab();
  const out = await book.rollN("agent", v, 3);
  assert.ok(out.length >= 1 && out.length <= 3);
  assert.equal(new Set(out).size, out.length, "rollN must produce uniques");
});

test("project realm option overrides 'fantasy' group", async () => {
  const v = buildVocab();
  v.addGroup("adjectives", "tarot", ["royal", "veiled"]);
  v.addGroup("nouns", "tarot", ["wheel", "tower"]);
  const book = buildBook();
  const name = await book.rollSeeded("project", v, "deterministic", { realm: "tarot" });
  // Result must come from tarot lists.
  const [adj, noun] = name.split("-");
  assert.ok(["Royal", "Veiled"].includes(adj), `unexpected adjective ${adj}`);
  assert.ok(["Wheel", "Tower"].includes(noun), `unexpected noun ${noun}`);
});

test("branch pattern emits prefix and lowercase parts", async () => {
  const book = buildBook();
  const v = buildVocab();
  const name = await book.roll("branch", v, { prefix: "feat" });
  assert.match(name, /^feat\/[a-z]+-[a-z]+-[a-z]+$/);
});

test("loadVocabularyFile parses the test fixture", async () => {
  const v = await loadVocabularyFile(TEST_VOCAB);
  assert.deepEqual(v.group("adjectives", "any").sort(), ["brisk", "quick"]);
  assert.deepEqual(v.group("nouns", "any").sort(), ["beacon", "forge"]);
  assert.equal(v.group("nouns", "missing"), null);
});

test("loadRecipeBook reads the real recipes.yaml", async () => {
  const book = await loadRecipeBook(REAL_RECIPES);
  assert.ok(book.has("project"));
  assert.ok(book.has("agent"));
  assert.ok(book.has("branch"));
  assert.ok(book.has("entity"));
  assert.deepEqual(book.requiredOptions("project"), ["realm"]);
  assert.deepEqual(book.requiredOptions("branch"), ["prefix"]);
});
