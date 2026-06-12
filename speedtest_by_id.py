#!/usr/bin/env python3
import json
import sys
import urllib.parse
import urllib.request

import speedtest


API_URL = "https://www.speedtest.net/api/js/servers"
HEADERS = {
    "User-Agent": "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 Chrome/126.0 Safari/537.36",
    "Accept": "application/json,text/javascript,*/*;q=0.01",
    "Referer": "https://www.speedtest.net/",
}


def fetch_servers(params):
    query = urllib.parse.urlencode(params)
    request = urllib.request.Request(f"{API_URL}?{query}", headers=HEADERS)
    with urllib.request.urlopen(request, timeout=10) as response:
        return json.load(response)


def find_server(server_id):
    queries = [
        {"engine": "js", "limit": 1000},
        {"engine": "js", "limit": 1000, "search": server_id},
    ]
    for params in queries:
        for server in fetch_servers(params):
            if server.get("id") == server_id:
                server["d"] = float(server.get("distance", 0))
                return server
    return None


def main():
    if len(sys.argv) != 2:
        print("usage: speedtest_by_id.py SERVER_ID", file=sys.stderr)
        return 2

    server_id = sys.argv[1]
    server = find_server(server_id)
    if server is None:
        print(f"server ID {server_id} was not found in speedtest.net API", file=sys.stderr)
        return 1

    client = speedtest.Speedtest(secure=True)
    client.servers = {server["d"]: [server]}
    best = client.get_best_server()
    download = client.download()
    upload = client.upload()

    print(json.dumps({
        "download": download,
        "upload": upload,
        "ping": best["latency"],
        "server": best,
    }))
    return 0


if __name__ == "__main__":
    sys.exit(main())
