// Catalog loader. Mirrors go/catalog.go.
//
// Reads catalog/projects.yaml. Save() round-trips the typed data back to YAML.
// Round-trip fidelity (comments, exact key order) is best-effort — see the
// note in go/catalog.go.

import { readFile, writeFile } from "node:fs/promises";
import yaml from "js-yaml";

export class Catalog {
  constructor(path, projects = []) {
    this.path = path;
    this.projects = projects;
  }

  resolve(name) {
    for (const p of this.projects) {
      if (p.id === name) return p;
      if (p.current_name === name) return p;
      for (const pn of p.prior_names || []) {
        if (pn.name === name) return p;
      }
    }
    return null;
  }

  byRealm(realm) {
    return this.projects.filter((p) => p.realm === realm);
  }

  byKind(kind) {
    return this.projects.filter((p) => p.kind === kind);
  }

  byStatus(status) {
    return this.projects.filter((p) => p.status === status);
  }

  claim(newName, opts = {}) {
    const { renamesOf, reason, retired } = opts;
    if (renamesOf) {
      const existing = this.resolve(renamesOf);
      if (!existing) {
        throw new Error(`claim: no project found for ${renamesOf}`);
      }
      existing.prior_names = existing.prior_names || [];
      if (existing.current_name && existing.current_name !== newName) {
        existing.prior_names.push({
          name: existing.current_name,
          retired: retired || todayIso(),
          reason: reason || "",
        });
      }
      existing.current_name = newName;
      return existing;
    }
    if (this.resolve(newName)) {
      throw new Error(`claim: ${newName} already in catalog`);
    }
    const fresh = {
      id: slugify(newName),
      current_name: newName,
      kind: "tool",
      realm: null,
      domain: null,
      repo: null,
      description: "",
      created: todayIso(),
      prior_names: [],
      status: "local-only",
    };
    this.projects.push(fresh);
    return fresh;
  }

  toYaml() {
    return yaml.dump(
      { projects: this.projects },
      { lineWidth: -1, noRefs: true, sortKeys: false, indent: 2 },
    );
  }

  async save(path) {
    const target = path || this.path;
    if (!target) throw new Error("catalog has no path");
    await writeFile(target, this.toYaml(), "utf8");
  }
}

export async function loadCatalog(path) {
  const text = await readFile(path, "utf8");
  const raw = yaml.load(text) || {};
  const projects = raw.projects || [];
  return new Catalog(path, projects);
}

function slugify(s) {
  return s
    .toLowerCase()
    .replace(/\.realm\.watch$/, "")
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");
}

function todayIso() {
  return new Date().toISOString().slice(0, 10);
}
