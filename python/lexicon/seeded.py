"""Cross-language deterministic index function.

Algorithm — must match Go's ``SeededIndex`` and JS's ``seededIndex``
byte-for-byte:

1. Compose ``seed_utf8 || big-endian uint64(slot)``
2. Hash with SHA-256
3. Read first 8 bytes of the digest, big-endian, as uint64
4. Return ``value % modulus``

Verified by ``tests/test_seeded.py`` against
``tests/fixtures/seeded-recipes.json``.
"""

from __future__ import annotations

import hashlib
import struct

__all__ = ["seeded_index"]


def seeded_index(seed: str, slot: int, modulus: int) -> int:
    """Deterministically map ``(seed, slot)`` into ``[0, modulus)``.

    Returns 0 if ``modulus <= 0`` (matches Go's guard).
    """
    if modulus <= 0:
        return 0
    if slot < 0:
        raise ValueError("slot must be a non-negative integer")
    h = hashlib.sha256()
    h.update(seed.encode("utf-8"))
    h.update(struct.pack(">Q", slot))
    digest = h.digest()
    (value,) = struct.unpack(">Q", digest[:8])
    return value % modulus
