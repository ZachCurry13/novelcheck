#!/bin/sh

set -e

# NovelCheck for KindleForge: downloads the NovelCheck Kindle app from its latest GitHub release, puts it in
# extensions/novelcheck-app and its launcher (a "book" called NovelCheck) in documents. The app asks for a NovelCheck
# address, name and password the first time it opens.

REL=https://github.com/ZachCurry13/novelcheck/releases/latest/download
APP=/mnt/us/extensions/novelcheck-app
TMPDIR=/mnt/us/KFPM-Temporary/novelcheck
rm -rf "$TMPDIR"
mkdir -p "$TMPDIR" "$APP" /mnt/us/documents

curl -fsSL -o "$TMPDIR/kindle-app.json" "$REL/kindle-app.json"
FILES=$(sed -n '/"files"/,/\]/p' "$TMPDIR/kindle-app.json" | grep -o '"[A-Za-z0-9._-]*"' | tr -d '"' | grep -v '^files$')
[ -n "$FILES" ] || exit 1

for f in $FILES; do
    curl -fsSL -o "$TMPDIR/$f" "$REL/kindle-app-$f"
    [ -s "$TMPDIR/$f" ] || exit 1
done
curl -fsSL -o "$TMPDIR/NovelCheck.sh" "$REL/kindle-app-NovelCheck.sh"
[ -s "$TMPDIR/NovelCheck.sh" ] || exit 1

# Everything came down: now copy it in. An existing config.js (a signed-in app) is kept.
for f in $FILES; do cp "$TMPDIR/$f" "$APP/$f"; done
[ -e "$APP/config.js" ] || echo 'var NC = {};' > "$APP/config.js"
cp "$TMPDIR/NovelCheck.sh" /mnt/us/documents/NovelCheck.sh
chmod +x /mnt/us/documents/NovelCheck.sh "$APP"/*.sh 2>/dev/null || true

rm -rf "$TMPDIR"
exit 0
