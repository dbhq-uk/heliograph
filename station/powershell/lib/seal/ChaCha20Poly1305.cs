// ChaCha20-Poly1305 (RFC 8439) for the PowerShell station's seal.
//
// WHY THIS IS HERE RATHER THAN VENDORED.  The design recommended NaCl.Core.
// It cannot be used: it is built on Span<T>, ReadOnlySpan<T>, stackalloc-into-
// span and System.Buffers across every file, and Span<T> does not exist on
// .NET Framework without a NuGet assembly - which the payload may not ship,
// because the whole proposition is plain text you can read before you run it.
// Windows PowerShell 5.1 compiles Add-Type source with the .NET Framework
// CodeDOM compiler, which is C# 5.
//
// BouncyCastle's ChaCha20Poly1305 was measured too: its dependency closure
// reaches ~6,000 lines and pulls in BigInteger through Arrays.cs, for an AEAD.
// That is the same mass objection that ruled BouncyCastle out to begin with.
//
// So the split is:
//
//   Poly1305  VENDORED - Chaos.NaCl's poly1305-donna. The 130-bit arithmetic
//             is the part with a subtle failure mode, and it is not ours.
//   ChaCha20  HERE - 20 rounds of add-rotate-xor over uint32. No secret-
//             dependent branch, no secret-dependent index, no bignum.
//   Framing   HERE - RFC 8439 section 2.8, about forty lines of padding and
//             two little-endian lengths.
//
// Every one of those is checked against the RFC's OWN vectors, not against a
// round trip: section 2.3.2 for the block function, 2.4.2 for the keystream,
// 2.5.2 for the MAC, and 2.8.2 for the whole AEAD. A round trip is satisfied
// by an implementation that is wrong in both directions, which is how
// MontgomeryCurve25519.KeyExchange passed for a whole afternoon.
//
// C# 5. No LINQ, no expression-bodied members, no interpolated strings, no
// Span. If this file stops compiling under -langversion:5 it stops working on
// the estate it exists for.

using System;
using Chaos.NaCl.Internal;

namespace Heliograph.Seal
{
    public static class ChaCha20Poly1305
    {
        public const int KeySize = 32;
        public const int NonceSize = 12;
        public const int TagSize = 16;

        // --- ChaCha20 -------------------------------------------------------

        private static uint Rotl(uint v, int n)
        {
            return (v << n) | (v >> (32 - n));
        }

        private static void QuarterRound(uint[] s, int a, int b, int c, int d)
        {
            s[a] += s[b]; s[d] ^= s[a]; s[d] = Rotl(s[d], 16);
            s[c] += s[d]; s[b] ^= s[c]; s[b] = Rotl(s[b], 12);
            s[a] += s[b]; s[d] ^= s[a]; s[d] = Rotl(s[d], 8);
            s[c] += s[d]; s[b] ^= s[c]; s[b] = Rotl(s[b], 7);
        }

        private static uint LoadLE(byte[] b, int off)
        {
            return (uint)b[off] | ((uint)b[off + 1] << 8) | ((uint)b[off + 2] << 16) | ((uint)b[off + 3] << 24);
        }

        private static void StoreLE(byte[] b, int off, uint v)
        {
            b[off] = (byte)v;
            b[off + 1] = (byte)(v >> 8);
            b[off + 2] = (byte)(v >> 16);
            b[off + 3] = (byte)(v >> 24);
        }

        // The 64-byte block. RFC 8439 section 2.3.
        //
        // The four constants are "expand 32-byte k" read as little-endian
        // uint32s. Spelled as numbers rather than derived from the string so
        // that a text encoding cannot get between the standard and this.
        public static byte[] Block(byte[] key, uint counter, byte[] nonce)
        {
            if (key == null || key.Length != KeySize)
                throw new ArgumentException("a ChaCha20 key is 32 bytes");
            if (nonce == null || nonce.Length != NonceSize)
                throw new ArgumentException("a ChaCha20 nonce is 12 bytes");

            uint[] init = new uint[16];
            init[0] = 0x61707865; init[1] = 0x3320646e; init[2] = 0x79622d32; init[3] = 0x6b206574;
            for (int i = 0; i < 8; i++) init[4 + i] = LoadLE(key, i * 4);
            init[12] = counter;
            for (int i = 0; i < 3; i++) init[13 + i] = LoadLE(nonce, i * 4);

            uint[] s = new uint[16];
            Array.Copy(init, s, 16);
            for (int i = 0; i < 10; i++)
            {
                QuarterRound(s, 0, 4, 8, 12);
                QuarterRound(s, 1, 5, 9, 13);
                QuarterRound(s, 2, 6, 10, 14);
                QuarterRound(s, 3, 7, 11, 15);
                QuarterRound(s, 0, 5, 10, 15);
                QuarterRound(s, 1, 6, 11, 12);
                QuarterRound(s, 2, 7, 8, 13);
                QuarterRound(s, 3, 4, 9, 14);
            }

            byte[] outb = new byte[64];
            for (int i = 0; i < 16; i++) StoreLE(outb, i * 4, s[i] + init[i]);
            return outb;
        }

        // XOR the keystream over the input. RFC 8439 section 2.4.
        public static byte[] Xor(byte[] key, uint counter, byte[] nonce, byte[] input)
        {
            byte[] outb = new byte[input.Length];
            int done = 0;
            uint ctr = counter;
            while (done < input.Length)
            {
                byte[] ks = Block(key, ctr, nonce);
                int n = input.Length - done;
                if (n > 64) n = 64;
                for (int i = 0; i < n; i++) outb[done + i] = (byte)(input[done + i] ^ ks[i]);
                done += n;
                ctr++;
            }
            return outb;
        }

        // --- Poly1305, over the vendored donna implementation ---------------

        // Public so RFC 8439 section 2.5.2 can be checked against it directly. A
        // vector that has to reach through reflection to find the thing it is
        // checking is a vector nobody runs.
        public static byte[] Poly1305Mac(byte[] oneTimeKey, byte[] message)
        {
            Array8<uint> k;
            k.x0 = LoadLE(oneTimeKey, 0);
            k.x1 = LoadLE(oneTimeKey, 4);
            k.x2 = LoadLE(oneTimeKey, 8);
            k.x3 = LoadLE(oneTimeKey, 12);
            k.x4 = LoadLE(oneTimeKey, 16);
            k.x5 = LoadLE(oneTimeKey, 20);
            k.x6 = LoadLE(oneTimeKey, 24);
            k.x7 = LoadLE(oneTimeKey, 28);
            byte[] tag = new byte[TagSize];
            Poly1305Donna.poly1305_auth(tag, 0, message, 0, message.Length, ref k);
            return tag;
        }

        // RFC 8439 section 2.6: the one-time key is the first 32 bytes of the
        // block at counter zero, so the counter for the data starts at one.
        public static byte[] OneTimeKey(byte[] key, byte[] nonce)
        {
            byte[] b = Block(key, 0, nonce);
            byte[] otk = new byte[32];
            Array.Copy(b, 0, otk, 0, 32);
            return otk;
        }

        // --- the AEAD, RFC 8439 section 2.8 ---------------------------------

        private static byte[] MacData(byte[] aad, byte[] ct)
        {
            int padA = (16 - (aad.Length % 16)) % 16;
            int padC = (16 - (ct.Length % 16)) % 16;
            byte[] m = new byte[aad.Length + padA + ct.Length + padC + 16];
            int o = 0;
            Array.Copy(aad, 0, m, o, aad.Length); o += aad.Length + padA;
            Array.Copy(ct, 0, m, o, ct.Length); o += ct.Length + padC;
            // Little-endian uint64 lengths. The seal's own metadata is
            // big-endian; these are not, because the RFC says so, and mixing
            // the two up is the kind of mistake a vector exists to catch.
            ulong la = (ulong)aad.Length, lc = (ulong)ct.Length;
            for (int i = 0; i < 8; i++) m[o + i] = (byte)(la >> (8 * i));
            for (int i = 0; i < 8; i++) m[o + 8 + i] = (byte)(lc >> (8 * i));
            return m;
        }

        public static byte[] Encrypt(byte[] key, byte[] nonce, byte[] plaintext, byte[] aad)
        {
            if (aad == null) aad = new byte[0];
            byte[] otk = OneTimeKey(key, nonce);
            byte[] ct = Xor(key, 1, nonce, plaintext);
            byte[] tag = Poly1305Mac(otk, MacData(aad, ct));
            byte[] outb = new byte[ct.Length + TagSize];
            Array.Copy(ct, 0, outb, 0, ct.Length);
            Array.Copy(tag, 0, outb, ct.Length, TagSize);
            return outb;
        }

        // Returns null when the tag does not verify. NEVER a partially
        // decrypted plaintext, and never a boolean beside an out parameter a
        // caller can read anyway: the only safe shape is one where forgetting
        // to check produces a null reference rather than a forged message.
        public static byte[] Decrypt(byte[] key, byte[] nonce, byte[] sealedBytes, byte[] aad)
        {
            if (aad == null) aad = new byte[0];
            if (sealedBytes == null || sealedBytes.Length < TagSize) return null;
            int ctLen = sealedBytes.Length - TagSize;
            byte[] ct = new byte[ctLen];
            Array.Copy(sealedBytes, 0, ct, 0, ctLen);
            byte[] have = new byte[TagSize];
            Array.Copy(sealedBytes, ctLen, have, 0, TagSize);

            byte[] otk = OneTimeKey(key, nonce);
            byte[] want = Poly1305Mac(otk, MacData(aad, ct));
            if (!ConstantTimeEquals(have, want)) return null;
            return Xor(key, 1, nonce, ct);
        }

        // No early return. A comparison that stops at the first differing byte
        // tells an attacker how much of a forged tag was right, one byte at a
        // time, and that is enough to build a valid one.
        public static bool ConstantTimeEquals(byte[] a, byte[] b)
        {
            if (a == null || b == null || a.Length != b.Length) return false;
            int diff = 0;
            for (int i = 0; i < a.Length; i++) diff |= a[i] ^ b[i];
            return diff == 0;
        }
    }
}
