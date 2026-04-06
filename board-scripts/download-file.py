#!/usr/bin/env python3
import sys
import urllib.request

if len(sys.argv) < 3:
    print("Usage: download-file.py <url> <output-path>")
    sys.exit(1)

url = sys.argv[1]
output = sys.argv[2]

headers = {"User-Agent": "Mozilla/5.0 (compatible; InternKim/1.0)"}
request = urllib.request.Request(url, headers=headers)

try:
    response = urllib.request.urlopen(request, timeout=15)
    content_type = response.headers.get("Content-Type", "")

    if "text/html" in content_type:
        print("ERROR: got HTML instead of file (blocked or wrong URL)")
        sys.exit(1)

    data = response.read()
    with open(output, "wb") as f:
        f.write(data)

    print("OK %d %s" % (len(data), content_type))
except Exception as e:
    print("ERROR: %s" % e)
    sys.exit(1)
