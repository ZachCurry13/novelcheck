#!/bin/sh
# Name: NovelCheck
# Author: NovelCheck
# Icon: data:image/png;base64,{{ICON}}
# DontUseFBInk

# NovelCheck for a jailbroken Kindle without KOReader. Tapping this "book" copies the app into the Kindle's
# own app runner (Mesquite), registers it the way KindleForge registers itself, and opens it. Nothing stays
# running after the app closes. Books it downloads go to documents/NovelCheck.

SRC="/mnt/us/extensions/novelcheck-app"
DST="/var/local/mesquite/NovelCheck"
DB="/var/local/appreg.db"
APP_ID="com.novelcheck.kindle"

[ -d "$SRC" ] || exit 1
mkdir -p /mnt/us/documents/NovelCheck

# KindleForge's helper (utild) lets the app save books and read the library list; start it if KindleForge put it here.
if [ -e /lib/ld-linux-armhf.so.3 ]; then UTILD=/var/local/kmc/UtildHF; else UTILD=/var/local/kmc/UtildSF; fi
if [ -e "$UTILD" ]; then
    chmod +x "$UTILD"
    "$UTILD"
fi

rm -rf "$DST"
cp -r "$SRC" "$DST"

sqlite3 "$DB" <<EOF
INSERT OR IGNORE INTO interfaces(interface) VALUES('application');
INSERT OR IGNORE INTO handlerIds(handlerId) VALUES('$APP_ID');
INSERT OR REPLACE INTO properties(handlerId,name,value) VALUES('$APP_ID','lipcId','$APP_ID');
INSERT OR REPLACE INTO properties(handlerId,name,value) VALUES('$APP_ID','command','/usr/bin/mesquite -l $APP_ID -c file://$DST/');
INSERT OR REPLACE INTO properties(handlerId,name,value) VALUES('$APP_ID','supportedOrientation','U');
EOF

# stop a copy still running, or it keeps showing the old files
lipc-set-prop com.lab126.appmgrd stop app://$APP_ID >/dev/null 2>&1
sleep 3 # as KindleForge does: the app manager needs a moment to see the new entry
nohup lipc-set-prop com.lab126.appmgrd start app://$APP_ID >/dev/null 2>&1 &
