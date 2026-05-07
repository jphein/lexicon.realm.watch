// Cross-language seeded RNG. Must match go/seeded.go and python/lexicon/seeded.py
// byte-for-byte.
//
// Algorithm:
//   1. Compose: utf8(seed) || big-endian uint64 of slot
//   2. Hash:    SHA-256
//   3. Read:    first 8 bytes of digest, big-endian, as uint64
//   4. Map:     value mod modulus
//
// Returns a Promise<number> because Web Crypto's SHA-256 is async. In Node we
// could compute synchronously via node:crypto, but we use the same async shape
// in both environments so callers don't need to branch.

const isNode =
  typeof process !== "undefined" &&
  process.versions != null &&
  process.versions.node != null;

let sha256;
if (isNode) {
  const { createHash } = await import("node:crypto");
  sha256 = (bytes) => {
    const h = createHash("sha256");
    h.update(bytes);
    return h.digest();
  };
} else {
  sha256 = async (bytes) => {
    const buf = await crypto.subtle.digest("SHA-256", bytes);
    return new Uint8Array(buf);
  };
}

function utf8Bytes(s) {
  if (isNode) {
    return Buffer.from(s, "utf8");
  }
  return new TextEncoder().encode(s);
}

function be8(slot) {
  // slot is a non-negative integer that may exceed Number.MAX_SAFE_INTEGER in
  // theory, but recipe slot counters stay tiny. Accept Number or BigInt.
  const out = new Uint8Array(8);
  let v = typeof slot === "bigint" ? slot : BigInt(slot);
  for (let i = 7; i >= 0; i--) {
    out[i] = Number(v & 0xffn);
    v >>= 8n;
  }
  return out;
}

function concatBytes(a, b) {
  const out = new Uint8Array(a.length + b.length);
  out.set(a, 0);
  out.set(b, a.length);
  return out;
}

function readBigUint64BE(bytes) {
  let v = 0n;
  for (let i = 0; i < 8; i++) {
    v = (v << 8n) | BigInt(bytes[i]);
  }
  return v;
}

// seededIndex returns a Promise resolving to an integer in [0, modulus).
// If modulus <= 0, resolves to 0 (matches Go).
export async function seededIndex(seed, slot, modulus) {
  if (modulus <= 0) return 0;
  const seedBytes = utf8Bytes(seed);
  const slotBytes = be8(slot);
  const composed = concatBytes(seedBytes, slotBytes);
  const digest = await sha256(composed);
  const v = readBigUint64BE(digest);
  return Number(v % BigInt(modulus));
}
