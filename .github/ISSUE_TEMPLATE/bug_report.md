---
name: Bug report
about: Something does not work as described
title: ''
labels: ''
assignees: ''

---

**Version**
The header shows `v<version>@<build timestamp>`. Paste it, plus the yt-dlp
version from Settings.

**What happened**
What you did, what you expected, what happened instead.

**The command line**
Open the log panel (bottom-right) and copy the `msg="yt-dlp exec"` line for the
failing download. Every invocation logs its full argv, so this is usually
enough to reproduce the problem by hand.

**Environment**
- Host OS and architecture
- yt-dlp version, and whether a PO Token provider is in use
- Whether downloads go through a proxy

**Log excerpt**
The relevant lines from the log panel, including any `level=ERROR` entries.
Cookie values are never logged; the file paths and argument lists are safe to
paste.
