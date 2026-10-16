# Fonts embedded in the HTML report

The HTML report embeds these files (as `data:` URLs) so it looks the same on every system
without loading anything from the network. They are IBM's own Latin-1 subsets, copied
unmodified from the official npm packages; see `LICENSE.txt` (SIL Open Font License 1.1,
Reserved Font Name "Plex").

| File | Package | SHA-256 |
|---|---|---|
| `IBMPlexSans-Regular-Latin1.woff2` | `@ibm/plex-sans` 1.1.0, `fonts/split/woff2/` | `b5ad7bd39f996144915f0ad9849a90183b27d8c28ad97ed98af5b1bebc51f6b1` |
| `IBMPlexSans-Medium-Latin1.woff2` | `@ibm/plex-sans` 1.1.0, `fonts/split/woff2/` | `b5610af04d0d4b5a14a621d96d974b993e945a065db1a8861918f69ef9321934` |
| `IBMPlexSans-SemiBold-Latin1.woff2` | `@ibm/plex-sans` 1.1.0, `fonts/split/woff2/` | `fff0ab3a88b0b4aa0b693e4f0201359a15183b08e3fa5696d1918d8f0ade8ad5` |
| `IBMPlexMono-Regular-Latin1.woff2` | `@ibm/plex-mono` 2.5.0, `fonts/split/woff2/` | `e8993d946649b9d01abb1ed06d574b19d8ea3e66b5c3948602db335c44c18e56` |
| `IBMPlexMono-Medium-Latin1.woff2` | `@ibm/plex-mono` 2.5.0, `fonts/split/woff2/` | `41201b658a328b9d00368215c2f1102770f80b15952ab82631e4006255e6365d` |

Characters outside Latin-1 (for example in scanned file names) fall back to the system
fonts listed after IBM Plex in `style.css`.
