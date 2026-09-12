// HKDF-SHA256, RFC 5869.
//
// Written here because .NET Framework has no HKDF - it arrived in .NET 5 - and
// because it is the one primitive on this page that is genuinely small: two
// HMACs and a loop. HMACSHA256 itself, which is the part worth not writing,
// ships with .NET Framework.
//
// Checked against RFC 5869 Test Case 1, both the PRK and the OKM. That vector
// earned its place: an earlier attempt at it "failed" because the IKM had been
// transcribed as 21 bytes where the RFC says 22, and a round trip would have
// reported success in both directions.
//
// C# 5, for the reason given in HeliographChaCha20Poly1305.cs.

using System;
using System.Security.Cryptography;

namespace Heliograph.Seal
{
    public static class Hkdf
    {
        public static byte[] Extract(byte[] salt, byte[] ikm)
        {
            if (salt == null) salt = new byte[32];
            using (HMACSHA256 h = new HMACSHA256(salt))
            {
                return h.ComputeHash(ikm);
            }
        }

        public static byte[] Expand(byte[] prk, byte[] info, int length)
        {
            if (info == null) info = new byte[0];
            if (length < 0 || length > 255 * 32)
                throw new ArgumentException("HKDF-SHA256 can produce at most 8160 bytes");

            byte[] okm = new byte[length];
            byte[] t = new byte[0];
            int done = 0;
            byte counter = 1;
            using (HMACSHA256 h = new HMACSHA256(prk))
            {
                while (done < length)
                {
                    byte[] input = new byte[t.Length + info.Length + 1];
                    Array.Copy(t, 0, input, 0, t.Length);
                    Array.Copy(info, 0, input, t.Length, info.Length);
                    input[input.Length - 1] = counter;
                    t = h.ComputeHash(input);

                    int n = length - done;
                    if (n > t.Length) n = t.Length;
                    Array.Copy(t, 0, okm, done, n);
                    done += n;
                    counter++;
                }
            }
            return okm;
        }

        public static byte[] DeriveKey(byte[] ikm, byte[] salt, byte[] info, int length)
        {
            return Expand(Extract(salt, ikm), info, length);
        }
    }
}
