# NovelCheck as a KindleForge package

`install.sh` and `uninstall.sh` are the two scripts a KindleForge package needs, and `registry-entry.json` is its entry for
a package list. `install.sh` downloads the Kindle app from NovelCheck's latest GitHub release into
`/mnt/us/extensions/novelcheck-app` and puts the launcher "book" (NovelCheck) in `documents`. The app asks for the NovelCheck
address, name and password the first time it opens (app 1.14.0 and later). An existing `config.js` is kept.

`uninstall.sh` removes the app and the launcher, and leaves the books the app downloaded (`documents/NovelCheck`).

The old KindleForge package list (`KindleTweaks/Repository`) is archived; its successor, KPM, takes `.kpkg` packages.
Nothing here has been submitted to either.
