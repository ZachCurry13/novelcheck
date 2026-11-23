#!/bin/sh
# Name: NovelCheck Check
# Author: NovelCheck
# DontUseFBInk

# Tap this "book" if the NovelCheck app doesn't work. It tests what the app needs on this Kindle, writes the result
# to documents/NovelCheck-check.txt (open it from the library) and sends it to NovelCheck's bug log. Changes nothing.

SERVER={{SERVER}}
NCUSER={{USER}}
NCKEY={{KEY}}
OUT=/mnt/us/documents/NovelCheck-check.txt
H1="x-auth-user: $NCUSER"
H2="x-auth-key: $NCKEY"

{
echo "NovelCheck check, app {{VERSION}}"
echo "== date"; date
echo "== firmware"; cat /etc/prettyversion.txt 2>/dev/null || cat /etc/version.txt 2>&1
echo "== hard-float (needs UtildHF)"; ls /lib/ld-linux-armhf.so.3 2>&1
echo "== KindleForge helper (utild): saves books and reads the library"; ls -l /var/local/kmc/UtildHF /var/local/kmc/UtildSF 2>&1
ps | grep -i [u]tild 2>&1
echo "== tools"; for t in curl sqlite3 lipc-set-prop mesquite; do printf '%s: ' "$t"; command -v "$t" || echo MISSING; done
echo "== app files"; ls /mnt/us/extensions/novelcheck-app 2>&1; ls /var/local/mesquite/NovelCheck 2>&1 | head -3
echo "== registered"; sqlite3 /var/local/appreg.db "SELECT name, value FROM properties WHERE handlerId='com.novelcheck.kindle';" 2>&1
echo "== library database"; ls -l /var/local/cc.db* 2>&1
cp /var/local/cc.db /tmp/nc_check.db 2>&1 && sqlite3 /tmp/nc_check.db "SELECT p_cdeType, COUNT(*), SUM(p_location IS NULL) FROM Entries WHERE p_type='Entry:Item' GROUP BY p_cdeType;" 2>&1
rm -f /tmp/nc_check.db*
echo "== downloads folder"; ls -la /mnt/us/documents/NovelCheck 2>&1 | head -10
echo "== reaching NovelCheck at $SERVER"
curl -sS -m 20 -o /dev/null -w "plain: http=%{http_code}\n" "$SERVER/kosync/plugin/version" -H "$H1" -H "$H2" 2>&1
echo "== free space"; df -h /mnt/us 2>&1 | tail -1
echo "== done"
} > "$OUT" 2>&1

# Send it to the bug log (the JSON is built by hand: busybox has no JSON tool).
TEXT=$(tr -d '\000-\010\013\014\016-\037' < "$OUT" | awk '{gsub(/\\/,"\\\\"); gsub(/"/,"\\\""); gsub(/\t/," "); printf "%s\\n", $0}')
curl -sS -m 20 -X POST "$SERVER/kosync/plugin/debug" -H "$H1" -H "$H2" -H "Content-Type: application/json" \
    --data "{\"device\":\"Kindle app\",\"version\":\"{{VERSION}}\",\"text\":\"$TEXT\"}" >/dev/null 2>&1 && SENT="Sent to NovelCheck." || SENT="Couldn't send it (no Wi-Fi?)."

lipc-set-prop com.lab126.scanner doFullScan 1 2>/dev/null
lipc-set-prop com.lab126.pillow pillowAlert "{ \"clientParams\":{ \"alertId\":\"appAlert1\", \"show\":true, \"customStrings\":[ { \"matchStr\":\"alertTitle\", \"replaceStr\":\"NovelCheck Check\" }, { \"matchStr\":\"alertText\", \"replaceStr\":\"Done. $SENT The result is NovelCheck-check in your library.\" } ] } }" 2>/dev/null
