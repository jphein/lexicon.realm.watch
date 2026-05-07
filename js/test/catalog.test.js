// Catalog tests. Uses tests/fixtures/catalog-test.yaml so we don't depend on
// the live catalog/projects.yaml.

import test from "node:test";
import assert from "node:assert/strict";
import { mkdtemp, copyFile, readFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import { dirname, resolve, join } from "node:path";
import { tmpdir } from "node:os";

import { Catalog, loadCatalog } from "../src/catalog.js";

const here = dirname(fileURLToPath(import.meta.url));
const FIXTURE = resolve(here, "../../tests/fixtures/catalog-test.yaml");

test("loadCatalog reads the test fixture", async () => {
  const cat = await loadCatalog(FIXTURE);
  assert.equal(cat.projects.length, 3);
  const ids = cat.projects.map((p) => p.id).sort();
  assert.deepEqual(ids, ["clock", "dreamspace", "realmwatch"]);
});

test("Catalog.resolve finds by id", async () => {
  const cat = await loadCatalog(FIXTURE);
  const proj = cat.resolve("clock");
  assert.ok(proj);
  assert.equal(proj.current_name, "clock.realm.watch");
});

test("Catalog.resolve finds by current_name", async () => {
  const cat = await loadCatalog(FIXTURE);
  const proj = cat.resolve("clock.realm.watch");
  assert.ok(proj);
  assert.equal(proj.id, "clock");
});

test("Catalog.resolve finds by prior name", async () => {
  const cat = await loadCatalog(FIXTURE);
  const proj = cat.resolve("dreamscape");
  assert.ok(proj);
  assert.equal(proj.id, "dreamspace");
  assert.equal(proj.current_name, "dreamscape.realm.watch");
});

test("Catalog.resolve returns null for unknown name", async () => {
  const cat = await loadCatalog(FIXTURE);
  assert.equal(cat.resolve("nonsense"), null);
});

test("Catalog.byRealm filters by realm", async () => {
  const cat = await loadCatalog(FIXTURE);
  assert.equal(cat.byRealm("signal").length, 1);
  assert.equal(cat.byRealm("oracle").length, 1);
  assert.equal(cat.byRealm("none").length, 0);
});

test("Catalog.byStatus filters by status", async () => {
  const cat = await loadCatalog(FIXTURE);
  assert.equal(cat.byStatus("active").length, 2);
  assert.equal(cat.byStatus("local-only").length, 1);
});

test("Catalog.claim with renamesOf appends prior_name and updates current_name", async () => {
  const cat = await loadCatalog(FIXTURE);
  const updated = cat.claim("oracle.realm.watch", {
    renamesOf: "realmwatch",
    reason: "moved into family",
    retired: "2026-05-07",
  });
  assert.equal(updated.current_name, "oracle.realm.watch");
  assert.equal(updated.prior_names.length, 1);
  assert.equal(updated.prior_names[0].name, "realmwatch");
  assert.equal(updated.prior_names[0].reason, "moved into family");
  // resolve via either old or new should still find it
  assert.ok(cat.resolve("realmwatch"));
  assert.ok(cat.resolve("oracle.realm.watch"));
});

test("Catalog.claim adds a fresh entry when no rename", async () => {
  const cat = await loadCatalog(FIXTURE);
  const fresh = cat.claim("brand-new-thing", { reason: "" });
  assert.equal(fresh.current_name, "brand-new-thing");
  assert.equal(fresh.id, "brand-new-thing");
  assert.deepEqual(fresh.prior_names, []);
});

test("Catalog.claim refuses duplicate fresh claim", async () => {
  const cat = await loadCatalog(FIXTURE);
  assert.throws(() => cat.claim("clock"));
});

test("Catalog.toYaml round-trip is parseable and preserves count", async () => {
  const cat = await loadCatalog(FIXTURE);
  const yamlText = cat.toYaml();
  const dir = await mkdtemp(join(tmpdir(), "lexicon-cat-"));
  const path = join(dir, "out.yaml");
  await import("node:fs/promises").then((fs) =>
    fs.writeFile(path, yamlText, "utf8"),
  );
  const reloaded = await loadCatalog(path);
  assert.equal(reloaded.projects.length, cat.projects.length);
  const ids = reloaded.projects.map((p) => p.id).sort();
  assert.deepEqual(ids, ["clock", "dreamspace", "realmwatch"]);
});

test("Catalog.save writes to disk", async () => {
  const dir = await mkdtemp(join(tmpdir(), "lexicon-cat-save-"));
  const path = join(dir, "projects.yaml");
  await copyFile(FIXTURE, path);
  const cat = await loadCatalog(path);
  cat.claim("brand-new", { reason: "" });
  await cat.save();
  const text = await readFile(path, "utf8");
  assert.match(text, /brand-new/);
});
