# @jphein/lexicon

JS library for [lexicon.realm.watch](../). Themed name rolling and project
catalog access. Cross-language parity with the Go reference and the Python
sibling — same recipes, same option keys, same seeded-roll bytes.

Works in Node ≥20 and modern browsers. The seeded RNG uses Web Crypto's
`crypto.subtle.digest` in browsers and `node:crypto` in Node, behind the same
async API. **`seededIndex` returns a Promise** because Web Crypto SHA-256 is
async.

## Install

```sh
npm install @jphein/lexicon js-yaml
```

## Use

```js
import {
  loadVocabularyFiles,
  loadRecipeBook,
  loadCatalog,
  seededIndex,
} from "@jphein/lexicon";

const vocab = await loadVocabularyFiles([
  "vocabularies/realms.yaml",
  "vocabularies/adjectives.yaml",
  "vocabularies/nouns.yaml",
  "vocabularies/scientists.yaml",
  "vocabularies/creatures.yaml",
]);

const book = await loadRecipeBook("vocabularies/recipes.yaml");

const project = await book.roll("project", vocab, { realm: "fantasy" });
const agent = await book.roll("agent", vocab);
const branch = await book.roll("branch", vocab, { prefix: "feat" });

// Deterministic — must match Go and Python byte-for-byte.
const same = await book.rollSeeded("agent", vocab, "my-seed");

const cat = await loadCatalog("catalog/projects.yaml");
const realmwatch = cat.resolve("realmwatch");
const fantasyProjects = cat.byRealm("fantasy");
```

## Test

```sh
cd js
npm install
node --test test/
```
