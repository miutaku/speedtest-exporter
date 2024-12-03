# About
It is a prometheus exporter that checks the quality of your connection using speedtest.net.

# Specification

- Accesses speedtest-cli to obtain upload/download speeds and ping values.
- Be mindful of the scrape interval as values are retrieved with each access to the exporter.
- Retrieving speedtest results takes time (35 seconds with the usage below), so a frequency of at least once per minute is recommended.

# Usage

```shell
(*>△<)< curl http://127.0.0.1:8080/metrics | grep "speedtest_"
  % Total    % Received % Xferd  Average Speed   Time    Time     Time  Current
                                 Dload  Upload   Total   Spent    Left  Speed
100  7518    0  7518    0     0    209      0 --:--:--  0:00:35 --:--:--  1610
# HELP speedtest_download_speed_mbps Download speed in Mbps
# TYPE speedtest_download_speed_mbps gauge
speedtest_download_speed_mbps 263.6850094047989
# HELP speedtest_ping_ms Ping in milliseconds
# TYPE speedtest_ping_ms gauge
speedtest_ping_ms 9.142
# HELP speedtest_upload_speed_mbps Upload speed in Mbps
# TYPE speedtest_upload_speed_mbps gauge
speedtest_upload_speed_mbps 198.71385496437676
```
