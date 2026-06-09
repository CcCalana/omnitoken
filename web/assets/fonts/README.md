# Self-hosted fonts

All fonts in this directory use the SIL Open Font License 1.1. Runtime font
requests are local to `web/assets/fonts/`; no external font CDN is used.

| Family | Project source | Local file |
| --- | --- | --- |
| Smiley Sans v2.0.1 | https://github.com/atelier-anchor/smiley-sans | `smiley-sans-display-subset.woff2` |
| Hanken Grotesk | https://github.com/google/fonts/tree/main/ofl/hankengrotesk | `hanken-grotesk-variable.woff2` |
| Noto Sans SC | https://github.com/google/fonts/tree/main/ofl/notosanssc | `noto-sans-sc-console-subset.woff2` |
| Geist Mono | https://github.com/vercel/geist-font | `geist-mono-variable.woff2` |

Smiley Sans and Noto Sans SC are subset to the characters currently present in
the static console source, plus ASCII, digits, and common Chinese punctuation.
The generated subset contains 417 characters, including 312 Han characters.
System fallbacks remain in the CSS stacks for names or content outside that set.

The corresponding OFL texts are kept in `licenses/`.
