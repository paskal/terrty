#!/bin/sh

# read URL from input
url=$1

if [ -z "$url" ]; then
  echo "This script checks oldest sitemap entry from URL given as an argument"
  echo "and gives non-zero exit code in case of problems."
  echo "Please set URL for check as first argument for the script, for example:"
  echo "$0 https://favor-group.ru/sitemap.xml"
  exit 2
fi

# retrieve lastmod from sitemap.xml
lines=$(xidel $url -e //lastmod -s)

# check that we have any lines to check
if [ -z "$lines" ]; then
  echo "Haven't found lastmod entries in sitemap by URL $url"
  exit 101
fi

# check dates in files and get oldest one in unix time (seconds) format
oldest_renewal=$(echo $lines | tr ' ' '\0' | xargs -I{} -0 date -D '%Y-%m-%dT%H:%M:%S' +%s -d {} | sort | head -1)

# get age in complete days for oldest lastmod entry
updated_days_ago=$((($(date +%s) - $oldest_renewal) / (60 * 60 * 24)))
alert_days_threshold=10

# compare oldest lastmod age in days with threshold
if [ "$updated_days_ago" -gt "$alert_days_threshold" ]; then
  echo "Oldest sitemap entry in $url is $updated_days_ago days old (alert threshold is $alert_days_threshold days)"
  exit 102
fi
