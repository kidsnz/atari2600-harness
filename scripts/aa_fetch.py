#!/usr/bin/env python3
"""aa_fetch.py — AtariAge スレッド採掘パイプライン（Wayback ファースト）。

AtariAge 本家は Cloudflare の bot チャレンジで直接取得できないため、
Wayback Machine の CDX API でスナップショットを列挙して取得する。

使い方:
    python3 scripts/aa_fetch.py <topic-url> <出力ディレクトリ> [-attachments] [-keep-raw] [-force]
    （既採掘 topic は自動 skip＝再採掘防止。-force で上書き再取得。掘る前の照会は aa_manifest.py --check）
    例: python3 scripts/aa_fetch.py \\
        https://forums.atariage.com/topic/85667-medieval-mayhem-2600/ \\
        ../reference/atariage/85667-medieval-mayhem

既定は**リーン運用**（溜め込まない）:
    - raw/ の HTML キャッシュは parse 後に削除（Wayback がアーカイブ＝いつでも再取得可能）。
      再実行を高速化したい時だけ -keep-raw。
    - 添付はダウンロードせず thread.md 末尾に一覧だけ記録。実物が要る時だけ -attachments。
    - 残すのは蒸留物（thread.md・gaps.md・手書きの notes）だけ。

出力:
    raw/pageNN.html   各ページのスナップショット（キャッシュ・再実行は差分のみ）
    thread.md         全ページの投稿を連結した Markdown（著者/日付/本文/添付）
    attachments/      添付ファイル（attachment.php と CDN 画像。取得できた分）
    gaps.md           取得できなかったページ/添付の一覧（Cookie や手動保存で補完する用）

フォールバック: 環境変数 AA_COOKIE にブラウザの Cookie ヘッダ値（cf_clearance 含む）を
入れると、Wayback に無いものを本家から直接取得する。
"""
import html
import json
import os
import re
import subprocess
import sys
import time
import urllib.parse

UA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Safari/537.36"


def curl(url, out=None, timeout=120):
    cmd = ["curl", "-sL", "-A", UA, "-m", str(timeout),
           "--retry", "4", "--retry-delay", "10", "--retry-all-errors", url]
    if out:
        cmd += ["-o", out, "-w", "%{http_code}"]
        r = subprocess.run(cmd, capture_output=True, text=True)
        return r.stdout.strip()
    r = subprocess.run(cmd, capture_output=True)
    return r.stdout


def cdx(url_pattern, status_filter=True, **params):
    q = {"url": url_pattern, "output": "json", "collapse": "urlkey", "limit": "2000"}
    if status_filter:
        q["filter"] = "statuscode:200"
    q.update(params)
    raw = curl("http://web.archive.org/cdx/search/cdx?" + urllib.parse.urlencode(q))
    try:
        rows = json.loads(raw)
    except Exception:
        return []
    return rows[1:] if rows else []


def page_no(url):
    m = re.search(r"/page[/-](\d+)/?$|/page/(\d+)/$", url.rstrip("/") )
    if m:
        return int(m.group(1) or m.group(2))
    return 1 if re.search(r"/topic/[^/]+/?$", url.rstrip("/")) else None


def discover_pages(topic_url):
    """CDX で全ページの最新スナップショットを列挙（新旧ドメイン両方）。"""
    slug = re.search(r"/topic/([^/]+)", topic_url).group(1)
    best = {}  # page -> (timestamp, original_url)
    for host in (f"forums.atariage.com/topic/{slug}*",
                 f"atariage.com/forums/topic/{slug}*"):
        for row in cdx(host):
            ts, orig = row[1], row[2]
            if "?" in orig:  # クエリ付き（embed/sort 等）は除外
                continue
            n = page_no(orig)
            if n is None:
                continue
            if n not in best or ts > best[n][0]:
                best[n] = (ts, orig)
    return best


CONTENT_RE = re.compile(r'<div data-role="commentContent"[^>]*>(.*?)</div>\s*</div>', re.S)
AUTHOR_RE = re.compile(r'/profile/\d+-([\w\-]+)/', re.S)  # プロフィール URL のスラッグ＝確実
TIME_RE = re.compile(r'<time[^>]*datetime="([^"]+)"')
ATTACH_RE = re.compile(r'href="[^"]*?(?:/web/\d+[a-z_]*/)?(https?://[^"]*?(?:attachment\.php\?id=\d+|media\.invisioncic\.com/[^"]+|uploads/monthly[^"]+))"')
ATTACH_NAME_RE = re.compile(r'attachment\.php\?id=(\d+)"[^>]*>\s*(?:<[^>]+>\s*)*([^<]{1,80})<')


def strip_html(h):
    h = re.sub(r"<br ?/?>", "\n", h)
    h = re.sub(r"<blockquote[^>]*>", "\n> ", h)
    h = re.sub(r"</blockquote>", "\n", h)
    h = re.sub(r"<[^>]+>", " ", h)
    h = html.unescape(h)
    h = re.sub(r"[ \t]+", " ", h)
    return re.sub(r"\n{3,}", "\n\n", h).strip()


def parse_page(path):
    s = open(path, encoding="utf-8", errors="ignore").read()
    posts = []
    # IPB4: article 単位で著者・時刻・本文を拾う
    arts = re.split(r'<article', s)[1:]
    for a in arts:
        c = CONTENT_RE.search(a)
        if not c:
            continue
        au = AUTHOR_RE.search(a)
        tm = TIME_RE.search(a)
        posts.append({
            "author": au.group(1) if au else "?",
            "time": tm.group(1)[:10] if tm else "?",
            "text": strip_html(c.group(1)),
        })
    atts = []
    for m in ATTACH_RE.finditer(s):
        u = m.group(1)
        u = re.sub(r"^https?://web\.archive\.org/web/\d+[a-z_]*/", "", u)
        atts.append(u)
    names = {m.group(1): m.group(2).strip() for m in ATTACH_NAME_RE.finditer(s)}
    return posts, sorted(set(atts)), names


def fetch_attachment(url, outdir, names, gaps):
    """添付を Wayback の素形（id_）→ AA_COOKIE 直接 の順で取得。"""
    aid = None
    m = re.search(r"attachment\.php\?id=(\d+)", url)
    if m:
        aid = m.group(1)
        fname = names.get(aid, f"attachment_{aid}")
        fname = re.sub(r"[^\w.\-]", "_", fname) or f"attachment_{aid}"
        if not re.search(r"\.\w{1,4}$", fname):
            fname += ".bin"
        fname = f"{aid}_{fname}"
    else:
        fname = re.sub(r"[^\w.\-]", "_", url.rsplit("/", 1)[-1])[:80]
    out = os.path.join(outdir, fname)
    if os.path.exists(out) and os.path.getsize(out) > 0:
        return True
    # 添付スナップショットは 301（実体へのリダイレクト）が普通 → status フィルタ無しで列挙し
    # replay URL（-L でリダイレクト追跡）から取得する。
    rows = cdx(url.replace("https://", "").replace("http://", ""), status_filter=False, limit="6")
    for row in rows:
        ts, orig = row[1], row[2]
        code = curl(f"http://web.archive.org/web/{ts}/{orig}", out=out)
        if code == "200" and os.path.exists(out) and os.path.getsize(out) > 200:
            return True
        time.sleep(1.5)
    cookie = os.environ.get("AA_COOKIE")
    if cookie:
        r = subprocess.run(["curl", "-sL", "-A", UA, "-H", f"cookie: {cookie}",
                            "-m", "90", url, "-o", out, "-w", "%{http_code}"],
                           capture_output=True, text=True)
        if r.stdout.strip() == "200" and os.path.getsize(out) > 200:
            return True
    if os.path.exists(out):
        os.remove(out)
    gaps.append(f"attachment: {url} ({names.get(aid, '?') if aid else '?'})")
    return False


def already_mined(topic_url, outdir):
    """reference/atariage に同じ topic_id の notes.ja.md が在れば既採掘（再採掘の機械的防止）。

    判定は **topic_id（URL の数字）**。slug 名が違っても同じスレなら検出する。
    outdir の親（= reference/atariage）に `<topic_id>-*/notes.ja.md` が在れば既採掘とみなす。
    """
    import glob
    m = re.search(r"/topic/(\d+)", topic_url)
    if not m:
        return None
    tid = m.group(1)
    refroot = os.path.dirname(os.path.abspath(outdir.rstrip("/")))
    hits = glob.glob(os.path.join(refroot, tid + "-*", "notes.ja.md"))
    return (tid, hits[0]) if hits else None


def main():
    if len([a for a in sys.argv[1:] if not a.startswith("-")]) != 2:
        print(__doc__)
        sys.exit(2)
    want_atts = "-attachments" in sys.argv
    keep_raw = "-keep-raw" in sys.argv
    force = "-force" in sys.argv
    args = [a for a in sys.argv[1:] if not a.startswith("-")]
    topic_url, outdir = args[0], args[1]
    mined = already_mined(topic_url, outdir)
    if mined and not force:
        print(f"ALREADY MINED topic {mined[0]} → {mined[1]}")
        print("skip（再採掘するなら -force）。掘る前の照会は scripts/aa_manifest.py --check <url> でも可。")
        sys.exit(0)
    raw = os.path.join(outdir, "raw")
    attdir = os.path.join(outdir, "attachments")
    os.makedirs(raw, exist_ok=True)
    os.makedirs(attdir, exist_ok=True)
    gaps = []

    pages = discover_pages(topic_url)
    if not pages:
        sys.exit("no snapshots found via CDX")
    maxp = max(pages)
    print(f"CDX: {len(pages)} pages archived (max page {maxp})")
    for n in range(1, maxp + 1):
        if n not in pages:
            gaps.append(f"page {n}: no snapshot")

    md = [f"# {topic_url}\n"]
    total_posts = 0
    all_atts, names = [], {}
    for n in sorted(pages):
        ts, orig = pages[n]
        path = os.path.join(raw, f"page{n:02d}.html")
        if not (os.path.exists(path) and os.path.getsize(path) > 10000):
            ok = False
            cands = [(ts, orig)]
            for row in cdx(orig.replace("https://", "").replace("http://", ""),
                           collapse="timestamp:6", limit="6"):
                cands.append((row[1], row[2]))
            for cts, corig in cands:
                code = curl(f"http://web.archive.org/web/{cts}/{corig}", out=path, timeout=120)
                if code == "200" and os.path.getsize(path) > 10000:
                    ok = True
                    break
                time.sleep(1)
            if not ok:
                gaps.append(f"page {n}: fetch failed")
                continue
            time.sleep(1)
        posts, atts, nm = parse_page(path)
        names.update(nm)
        all_atts += atts
        total_posts += len(posts)
        md.append(f"\n## page {n} ({len(posts)} posts, snapshot {ts[:8]})\n")
        for p in posts:
            md.append(f"\n### {p['author']} — {p['time']}\n\n{p['text']}\n")
        print(f"page {n}: {len(posts)} posts, {len(atts)} attachment links")

    uniq = sorted(set(all_atts))
    got = 0
    if want_atts:
        for u in uniq:
            if fetch_attachment(u, attdir, names, gaps):
                got += 1
    # 添付一覧は常に thread.md へ記録（後から -attachments で取り直せる）
    md.append("\n## attachments (" + str(len(uniq)) + ")\n")
    for u in uniq:
        aid = re.search(r"attachment\.php\?id=(\d+)", u)
        nm = names.get(aid.group(1), "") if aid else ""
        md.append(f"- {u} {nm}")
    open(os.path.join(outdir, "thread.md"), "w").write("\n".join(md))
    if not keep_raw:
        import shutil
        shutil.rmtree(raw, ignore_errors=True)
    if not want_atts and not os.listdir(attdir):
        os.rmdir(attdir)
    open(os.path.join(outdir, "gaps.md"), "w").write(
        "# Gaps\n" + ("\n".join(f"- {g}" for g in gaps) if gaps else "(none)") + "\n")
    print(f"\nDONE: {total_posts} posts / attachments {got}/{len(uniq)} / gaps {len(gaps)}")
    print(f"→ {outdir}/thread.md, attachments/, gaps.md")


if __name__ == "__main__":
    main()
