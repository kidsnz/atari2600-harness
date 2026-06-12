#!/usr/bin/env python3
"""aa_index.py — AtariAge フォーラムの全スレッド目録を Wayback から作る（deep dive 第1段）。

フォーラム一覧ページのスナップショットを CDX で列挙し、スレッドの
タイトル・URL・作者・返信数・閲覧数を抽出して 1 枚の CSV にする。
同じスレが複数スナップショットに出る場合は最大の閲覧数を採用（=最新値に近い）。

使い方:
    python3 scripts/aa_index.py <forum-url> <出力CSV>
    例: python3 scripts/aa_index.py \\
        https://forums.atariage.com/forum/50-atari-2600-programming/ \\
        ../reference/atariage/index-forum50.csv

出力 CSV: topic_url,title,author,replies,views,last_seen
閲覧数で降順ソートすれば「掘る価値ランキング」になる。
"""
import csv
import html
import json
import os
import re
import subprocess
import sys
import time
import urllib.parse

UA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Safari/537.36"


def curl(url, timeout=120):
    r = subprocess.run(["curl", "-sL", "-A", UA, "-m", str(timeout),
                        "--retry", "3", "--retry-delay", "8", "--retry-all-errors", url],
                       capture_output=True)
    return r.stdout.decode("utf-8", errors="ignore")


def cdx(url_pattern, **params):
    q = {"url": url_pattern, "output": "json", "filter": "statuscode:200",
         "collapse": "urlkey", "limit": "4000"}
    q.update(params)
    raw = curl("http://web.archive.org/cdx/search/cdx?" + urllib.parse.urlencode(q))
    try:
        rows = json.loads(raw)
    except Exception:
        return []
    return rows[1:] if rows else []


TOPIC_RE = re.compile(
    r'href="[^"]*?/topic/(\d+-[\w\-]+)/"[^>]*>\s*<span[^>]*>([^<]{1,200})</span>', re.S)
ROW_SPLIT = re.compile(r'<li[^>]*class="ipsDataItem[^"]*"')
NUM_RE = re.compile(r'([\d,.]+)[km]?\s*(?:replies|repl|views)', re.I)


def parse_index(html_text):
    """IPB4 のフォーラム一覧から (slug, title, author, replies, views) を抽出。"""
    out = []
    for chunk in ROW_SPLIT.split(html_text)[1:]:
        m = re.search(r'href="[^"]*?/topic/(\d+-[\w\-]+)/?"[^>]*>([^<]{1,250})<', chunk)
        if not m:
            continue
        slug = m.group(1)
        title = html.unescape(m.group(2)).strip()
        if not title or title.lower().startswith(("started by", "by ")):
            continue
        au = re.search(r'/profile/\d+-([\w\-]+)/', chunk)
        # 統計: <span ...>N</span> replies / views（表記ゆれに耐えるため近傍数値を順に拾う）
        stats = re.findall(r'ipsDataItem_stats[^>]*>.*?([\d][\d,.]*[km]?)\s*<', chunk, re.S)
        nums = []
        for t in re.findall(r'>([\d][\d,.]*[km]?)<', chunk):
            v = t.lower().replace(",", "")
            mult = 1
            if v.endswith("k"):
                mult, v = 1000, v[:-1]
            elif v.endswith("m"):
                mult, v = 1000000, v[:-1]
            try:
                nums.append(int(float(v) * mult))
            except ValueError:
                pass
        replies = nums[0] if nums else 0
        views = max(nums[1:3]) if len(nums) > 1 else 0
        out.append((slug, title, au.group(1) if au else "?", replies, views))
    return out


def main():
    if len(sys.argv) != 3:
        print(__doc__)
        sys.exit(2)
    forum_url, out_csv = sys.argv[1], sys.argv[2]
    slug = re.search(r"/forum/([^/]+)", forum_url).group(1)
    best = {}  # page_no -> (ts, orig)
    for host in (f"forums.atariage.com/forum/{slug}*", f"atariage.com/forums/forum/{slug}*"):
        for row in cdx(host):
            ts, orig = row[1], row[2]
            if "?" in orig:
                continue
            m = re.search(r"/page/(\d+)/?$", orig.rstrip("/"))
            n = int(m.group(1)) if m else (1 if re.search(r"/forum/[^/]+/?$", orig.rstrip("/")) else None)
            if n is None:
                continue
            if n not in best or ts > best[n][0]:
                best[n] = (ts, orig)
    print(f"CDX: {len(best)} index pages archived (max {max(best) if best else 0})")

    topics = {}  # slug -> dict
    for n in sorted(best):
        ts, orig = best[n]
        body = curl(f"http://web.archive.org/web/{ts}/{orig}")
        rows = parse_index(body)
        for tslug, title, author, replies, views in rows:
            cur = topics.get(tslug)
            if cur is None or views > cur["views"]:
                topics[tslug] = {"title": title, "author": author,
                                 "replies": replies, "views": views, "seen": ts[:8]}
        print(f"index page {n} ({ts[:8]}): {len(rows)} topics (cumulative {len(topics)})")
        time.sleep(1)

    os.makedirs(os.path.dirname(out_csv) or ".", exist_ok=True)
    with open(out_csv, "w", newline="") as f:
        w = csv.writer(f)
        w.writerow(["topic_url", "title", "author", "replies", "views", "last_seen"])
        for tslug, d in sorted(topics.items(), key=lambda kv: -kv[1]["views"]):
            w.writerow([f"https://forums.atariage.com/topic/{tslug}/",
                        d["title"], d["author"], d["replies"], d["views"], d["seen"]])
    print(f"\nDONE: {len(topics)} topics → {out_csv} (views 降順)")


if __name__ == "__main__":
    main()
