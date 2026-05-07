// @jphein/lexicon — themed name rolling and project catalog for the
// realm.watch family. JS parity with the Go reference at github.com/jphein/lexicon.realm.watch/go.
//
// Public surface (named exports):
//   roll(recipe, vocabulary, options)
//   rollSeeded(recipe, vocabulary, seed, options)
//   rollN(recipe, vocabulary, n, options)
//   loadVocabularyFile(path)
//   loadVocabularyFiles(paths)
//   loadRecipeBook(path)
//   loadCatalog(path)
//   seededIndex(seed, slot, modulus)
//   Vocabulary, RecipeBook, Catalog (classes)

export { seededIndex } from "./seeded.js";
export {
  Vocabulary,
  loadVocabularyFile,
  loadVocabularyFiles,
} from "./vocabulary.js";
export { RecipeBook, loadRecipeBook } from "./recipe.js";
export { Catalog, loadCatalog } from "./catalog.js";

import { loadRecipeBook } from "./recipe.js";

// Convenience wrappers that take a RecipeBook OR a path. If a string is
// given it's treated as a path to recipes.yaml.
async function asBook(bookOrPath) {
  if (typeof bookOrPath === "string") {
    return loadRecipeBook(bookOrPath);
  }
  return bookOrPath;
}

export async function roll(bookOrPath, recipeName, vocabulary, options = {}) {
  const book = await asBook(bookOrPath);
  return book.roll(recipeName, vocabulary, options);
}

export async function rollSeeded(
  bookOrPath,
  recipeName,
  vocabulary,
  seed,
  options = {},
) {
  const book = await asBook(bookOrPath);
  return book.rollSeeded(recipeName, vocabulary, seed, options);
}

export async function rollN(
  bookOrPath,
  recipeName,
  vocabulary,
  n,
  options = {},
) {
  const book = await asBook(bookOrPath);
  return book.rollN(recipeName, vocabulary, n, options);
}
