// Vocabulary loader. Mirrors go/vocabulary.go.
//
// A vocabulary YAML file has the shape:
//   <category>:
//     <group>:
//       description: <text>
//       words: [<word>, ...]
//
// Multiple files can be merged into one Vocabulary by calling loadVocabularyFile
// once per file and combining via addGroup.

import { readFile } from "node:fs/promises";
import yaml from "js-yaml";

export class Vocabulary {
  constructor(categories = {}) {
    this.categories = categories;
  }

  group(category, group) {
    const cat = this.categories[category];
    if (!cat) return null;
    const g = cat[group];
    if (!g) return null;
    return g.words || [];
  }

  listCategories() {
    return Object.keys(this.categories);
  }

  listGroups(category) {
    const cat = this.categories[category];
    if (!cat) return [];
    return Object.keys(cat);
  }

  addGroup(category, group, words) {
    if (!this.categories[category]) this.categories[category] = {};
    this.categories[category][group] = { words: [...words] };
  }

  merge(other) {
    for (const cat of Object.keys(other.categories)) {
      for (const grp of Object.keys(other.categories[cat])) {
        this.addGroup(cat, grp, other.categories[cat][grp].words || []);
      }
    }
  }
}

export async function loadVocabularyFile(path) {
  const text = await readFile(path, "utf8");
  const raw = yaml.load(text) || {};
  return new Vocabulary(raw);
}

export async function loadVocabularyFiles(paths) {
  const v = new Vocabulary();
  for (const p of paths) {
    const part = await loadVocabularyFile(p);
    v.merge(part);
  }
  return v;
}
