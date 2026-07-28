# Node identity — its own namespace, and why the word count is load-bearing

**Authorised by JP, 2026-07-28:** *"yes fix and expand sigil and lexicon pls"* — names must be unique
by **guarantee**, not by low probability, and *"beautiful and in the realm we want."*

Two separate problems were tangled together here. Solving only the obvious one would have left the
other in place, so this document states both, and the second one is the surprise.

---

## 1. The collision problem — and why "add more words" is the wrong fix

Measured over the full `u8` id space (256 ids), with the existing 20-adjective × 20-noun corpus:

- **noun alone:** 20 distinct → **236 collisions** (Herald 16 ids, Forge 16, Aegis 15)
- **adjective + noun:** 163 distinct of 400 combinations → **93 collisions**

The instinct is that the corpus is too small. **It isn't. The arithmetic is wrong.** Sweeping divisor
choices against the real mapping — `adj[seed % A]`, `noun[(seed >> 8) % N]`, `seed = id × 2654435761`:

| adj × noun | combinations | distinct / 256 | collisions |
|---|---|---|---|
| 20 × 20 | 400 | 163 | **93** |
| **24 × 25** | 600 | 55 | **201** ← *worse with 50 % more words* |
| 28 × 25 | 700 | 237 | 19 |
| 33 × 31 | 1023 | 236 | 20 |
| 40 × 40 | 1600 | 249 | 7 |
| **32 × 16** | **512** | **256** | **0** |
| **32 × 32** | **1024** | **256** | **0** |

**`32 × 16` = 512 combinations achieves zero collisions while `40 × 40` = 1600 still collides seven
times.** Three times the vocabulary, still broken. So corpus size is not the lever.

### The mechanism — and the limit of the explanation
With `A = 2^k`, `seed % A` selects the seed's **low k bits** and `(seed >> 8) % N` selects **bits
8…8+k−1**. Those bit *ranges* don't overlap, which is why powers of two behave far better than
arbitrary moduli: non-power-of-two moduli *mix* bits across the whole word and correlate the two
indices, which is why `24 × 25` is catastrophic despite the counts being coprime. (Coprimality is the
wrong intuition and was my first guess; the numbers refuted it.)

One algebraic half **is** provable, and it is why **32** specifically: `2654435761` is **odd**, so
`gcd(G, 32) = 1` and `id ↦ (id·G) mod 32` is a **bijection mod 32**. So 32 adjectives separate every
id-class mod 32 outright, leaving the noun to separate only the 8 ids inside each class.

> ⚠️ **The second half is NOT proved, and I overstated this at first.** Calling the two fields
> "disjoint" is too strong: **carries out of the low bits of `i·G` propagate into bits 8–12**, so the
> noun index is not independent of the adjective index and there is no clean algebraic argument that the
> 8 ids in a class always land on 8 distinct nouns. *(morpheus-sigil's correction, verified.)*
>
> **So the enumeration is doing real work — it is not decorating a theorem.** Injectivity is a property
> of the **tuple `(G, A, N)`**, established by exhaustive check, not by the bit-field picture. The
> picture explains *why to look at powers of two*; it does not license skipping the test.

### 🔓 Corollary: words are free, counts are not
Because injectivity depends only on `(G, A, N)` — **never on which words are in the lists** — the
vocabulary can be **re-curated freely**: swap a word, reorder, beautify, replace something JP dislikes.
**None of that can break uniqueness.** Only two things can:

1. changing a **count** away from 32, or
2. letting a **reserved word** back in.

Those are exactly the two properties the sigil crate gates, which is why they are asserted as an
**equality** (`len == 32`) and an **empty intersection** rather than as bounds.

### What the guarantee is
**Not** a probability, and **not** a size inequality — `24 × 25` satisfies `600 ≥ 256` and collides 201
times. The id space is a `u8`, so **256 cases is a complete enumeration**: an exhaustive check that all
256 ids yield 256 distinct names *is* the proof, and it stays true as words are reserved later.

> **So the test is: enumerate all 256 ids, assert 256 distinct names, fail the build.** Same shape as
> smol's `DIAG_CORE_MAX ≤ DIAG_BUDGET` compile-time assertion — **make the failure unrepresentable
> rather than documented.**

### ⚠️ The count is therefore load-bearing
**Adding or removing one word from a node-identity list breaks uniqueness and renames every board.**
Indices are `% len`. This is why the vocabulary lives in a **size-locked group** (§3) rather than in the
general `fantasy` group, which other consumers are free to grow.

---

## 2. The namespace problem — identity must not share vocabulary with anything else

JP's instruction: node names must not reuse project vocabulary, naming **`Sigil`** and **`Crown`**
specifically. `crown` is the **gateway role**, so a board permanently called Crown is a genuine
ambiguity — and it cost real debugging time on 2026-07-28, when a roster entry read `Celestial Crown`
and the reader could not tell a *name* from a *role*.

Sweeping `docs/`, `ha/`, `site/` and the firmware rather than trusting a hand-listed set found the
problem was wider than the two words. **Six of the 25 fantasy nouns were project vocabulary:**

| Word | Also means | Why it's disqualifying |
|---|---|---|
| `crown` | the elected **gateway role** | JP's example. A log line about "the crown" becomes ambiguous |
| `beacon` | a **SMOLv1 wire frame** (`SMOLv1 BEACON`) | "Beacon missed a BEACON" is a real sentence |
| `forge` | the **version-name realm** in `names.rs` | collides with *provenance* — see §4 |
| `herald` | a Control Room feature **and** #197's toast transport | and it was the name of three devices |
| `oracle` | the adversarial-review role ("Oracle findings") | process vocabulary |
| `sigil` | the versioning system **and** a screen plugin | JP's example |

`hollow` was also dropped as a noun — it is an adjective in the same realm, so **"Hollow Hollow"** is a
reachable roll.

**The exclusion is data, not a habit.** [`vocabularies/reserved.yaml`](../../../vocabularies/reserved.yaml)
holds the reserved set grouped by *why* (roles, frames, namespaces, features, tools, workflow) so a
consumer can assert `node vocabulary ∩ reserved == ∅` and fail the build. A hand-filtered list rots
exactly like the hand-maintained entity lists behind smol issues
[#308](https://github.com/jphein/smol/issues/308)–[#310](https://github.com/jphein/smol/issues/310) —
**a self-reported manifest beats a hand-maintained list**, and a test beats a memory.

⚠️ **Apply the exclusion *before* the uniqueness assertion.** A test that computes distinctness over the
raw YAML passes while the live namespace still collides.

---

## 2b. 🔴 INVARIANT: a bare noun is never an identifier

**This is the invariant, not a mitigation — and no vocabulary size can replace it.**

The full sigil is unique: **256/256 distinct, 0 collisions.** The bare noun is not, and *cannot be*:

| identifier form | distinct over 256 ids | collisions |
|---|---|---|
| **full sigil** (`adj noun`) | **256** | **0** |
| bare noun, 32 nouns | 32 | **224** (max **9** ids share one noun) |
| bare noun, 16 nouns | 16 | **240** (max **18**) |

**Pigeonhole makes this permanent:** 256 ids over N nouns forces **≥ ⌈256/N⌉ ids per noun**. With 32
nouns that is **≥ 8 boards sharing every noun, guaranteed**. Buying noun-uniqueness with vocabulary
would need **N ≥ 256** — 256 nouns, which is absurd. **So the rule is the only fix available.**

> **INVARIANT — any surface too small for the full sigil MUST show a disambiguated short form: `noun +
> id`, or `adjective-initial + noun`. Never the bare noun.**

### This is the actual root cause of the three-Herald bug
`name_for_id()` has *always* returned a unique pair. Callers threw half of it away. And it is **not one
bad line — it is systemic**, which is precisely why it needs to be a stated rule rather than a fix:

`custom.rs:126` · `clock.rs:112` · `menu.rs:135` · `about.rs:92` · `ota_screen.rs:310` ·
`mesh_snake/mod.rs:488` · `finder.rs:223` · `finder.rs:246` — **eight call sites take `.1` and discard
the adjective.** Every one is a space-constrained surface, so every one was locally reasonable. The
72×40 OLED cannot render "Obsidian Aegis", so *something* must shorten — the bug is that the shortening
chose the **non-unique half**.

`finder.rs` goes further and **clips** the noun (`clip(name, 6)`, `clip(name, 8)`), so a fix that only
restores the adjective still needs the clip to be applied to a form that survives it.

### Curation constraint that falls out of it
Because short forms clip, **the noun set must stay distinct under truncation.** Verified for the shipped
32: distinct at **4, 5, 6 and 8** characters (morpheus-sigil independently confirmed 4 — one tighter
than I checked). This is *not* automatic — `citadel`/`citation` would collide at 6 — so **treat it as a
check when swapping any word**, not a happy accident. It is the one curation constraint that is *not*
covered by the counts-are-the-only-risk corollary above, because it depends on the letters.

### Why this settles 32 × 32 over 32 × 16
Both are collision-free on the **full sigil**, so the tiebreak is entirely about the **short form**:
32 nouns halve the bare-noun blast radius (**max 9 ids per noun instead of 18**). That is mitigation
while the invariant is the fix — but it is free mitigation, and the fleet grows.

---

## 3. The taxonomy — the durable part

`names.rs` already separates two namespaces deliberately: node names come from FANTASY, **version**
names from FORGE, *"so a build's identity reads in a deliberately different vocabulary from a node's —
provenance is never confused with identity at a glance."* This generalises that principle one level out.

| Namespace | Names | Vocabulary | Rule |
|---|---|---|---|
| **Identity** | a node — *which board is this?* | `adjectives.fleet` + `nouns.fleet` (**size-locked 32 × 32**) | never reuses any word below |
| **Provenance** | a build — *which firmware is this?* | the **forge** realm (`Riveted Furnace`) | deliberately a different register: industrial, not mythic |
| **Roles** | a function — *what is this board doing?* | `crown`, `gateway`, `leaf` | plain English, never a name |
| **Frames** | a wire message | `BEACON`, `HELLO`, `DIAG`… | ALL-CAPS on the wire, never a name |
| **Features** | a capability | `Bard`, `Herald`, `Familiar`, `Cast` | product words, never a name |
| **Tools** | a program | `sigil`, `lexicon`, `meshscope` | project words, never a name |

**The rule in one line: identity must not share a vocabulary with roles, versions, frames, features or
tools.** Everything above is one of those, so everything above is reserved.

**The test this implies** — and it is the thing that stops this recurring when someone ships a feature
called Ember: **when you coin a project term, add it to `reserved.yaml` in the same change.** The build
then tells the *next* person, instead of a reader discovering it in a log six weeks later.

---

## 4. What was actually changed

- **New `fleet` group** in `adjectives.yaml` and `nouns.yaml` — **32 words each**, in the existing
  fantasy voice, marked size-locked with the arithmetic reasoning inline.
- **New `reserved.yaml`** — the exclusion set as data, grouped by why.
- **The `fantasy` group is untouched.** That was deliberate and it had a concrete payoff: the `project`
  recipe rolls `{adjective:cap}-{noun:cap}` from **fantasy**, and
  `tests/fixtures/seeded-recipes.json` is a **cross-language parity contract** whose algorithm is
  `SeededIndex(…) mod modulus` — *the modulus is the list length*. Editing `fantasy` would have changed
  every expected name and forced a fixture regeneration across Go, Python and JS. **Putting node
  identity in its own group avoided touching that contract at all.** `lexicon validate` → `OK`;
  `go test ./...` → all pass.

**Adjectives added (4):** `argent`, `ashen`, `seraphic`, `umbral`.
**Nouns added (14):** `bastion`, `brazier`, `cairn`, `chalice`, `citadel`, `diadem`, `glyph`,
`lodestar`, `obelisk`, `orrery`, `sanctum`, `spire`, `talisman`, `vigil`.

Chosen for how they *sound*, since JP says these out loud, and for pairing across the whole adjective
set rather than in isolation — *Obsidian Aegis*, *Seraphic Dominion*, *Mystic Chalice*, *Ashen Vigil*,
*Radiant Obelisk*, *Celestial Orrery*, *Kindled Brazier*.

### Verified
```
32 × 32 → 256 / 256 distinct over the full u8 space, 0 collisions
```

---

## 5. For JP to overrule if he disagrees

Four judgement calls, stated so they can be reversed cheaply:

1. **🔴 Every board gets renamed, once.** Unavoidable — indices are `% len`, so any corpus change
   re-maps every id. **`Jade Herald` and `Draconic Dominion` do not survive** (and `herald` is reserved
   regardless). This is the strongest argument for doing it **once with headroom** rather than
   incrementally: each future addition would rename the fleet again. *Sample of the new mapping:*
   id5 → *Obsidian Aegis* · id8 → *Eldritch Jewel* · id50 → *Mystic Chalice* · id51 → *Ashen Vigil* ·
   id122 → *Somber Vigil*.
2. **I kept node identity in the fantasy realm** rather than proposing a new one. The names are already
   on hardware, in HA and in the docs, JP likes them, and the reserved-word problem exists in *any*
   realm — so a realm change would be churn without benefit. Reversible: the `fleet` group can be
   re-voiced without touching the taxonomy.
3. **`herald` and `oracle` are reserved even though they are lovely words.** Both are genuinely
   overloaded today. If a feature is ever renamed, they can return — the reserved set is data.
4. **32 × 32, not 32 × 16 and not 64.** Both smaller options are collision-free on the *full* sigil, so
   the tiebreak is the **short form** (§2b): 32 nouns halve the bare-noun blast radius, max 9 ids per
   noun instead of 18. Going wider (64) would mean a bigger one-time rename for headroom nobody has
   asked for. *If 32 genuinely good nouns had not been reachable after exclusions, 16 good words beat 32
   padded ones — the invariant matters more than the count.*

---

*Author: Nebula, 2026-07-28. Counts and the mapping confirmed against
`smol/rust/clock/src/net/names.rs`; the Rust binding and smol's integration are morpheus-sigil's.*
