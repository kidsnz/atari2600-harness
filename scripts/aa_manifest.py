#!/usr/bin/env python3
"""aa_manifest.py — AtariAge 採掘マニフェストの生成・照会（再採掘の恒久防止）。

採掘済みの「真実」はファイルシステム = `reference/atariage/<topic_id>-<slug>/notes.ja.md`
が存在するスレ。本スクリプトはそれを走査して 1 枚の台帳 `reference/atariage/MINED.csv` に
再生成する（**冪等・いつでもファイルから作り直せる＝陳腐化しない**）。

使い方:
    cd harness
    # 台帳を再生成（採掘の後に毎回これを実行）
    python3 scripts/aa_manifest.py
    # ある URL / topic_id が既採掘か照会（採掘の前に必ずこれ）
    python3 scripts/aa_manifest.py --check https://forums.atariage.com/topic/250014-tiatracker/
    python3 scripts/aa_manifest.py --check 250014

照会の規約: topic_id（URL 中の数字）が既採掘なら "MINED ..." を表示して exit 1、
未採掘なら "NEW ..." を表示して exit 0。スクリプトから `if aa_manifest --check <url>; then 採掘; fi`。

真実はディレクトリの有無なので、slug 名が違っても topic_id が同じなら重複と判定できる。
notes.ja.md がまだ無い（蒸留途中）のディレクトリは「未採掘」扱い＝採掘対象に残る。
"""
import csv
import os
import re
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.normpath(os.path.join(HERE, "..", "..", "reference", "atariage"))


def topic_id_of(name_or_url):
    """ディレクトリ名 / URL / 生の番号から topic_id（数字）を取り出す。"""
    s = str(name_or_url)
    m = re.search(r"/topic/(\d+)", s)        # URL 形式
    if m:
        return m.group(1)
    m = re.match(r"(\d+)", s)                 # "250014-tiatracker" や "250014"
    if m:
        return m.group(1)
    return None


def scan():
    """notes.ja.md を持つディレクトリを走査して採掘済みエントリの一覧を返す。"""
    rows = []
    if not os.path.isdir(ROOT):
        return rows
    for name in sorted(os.listdir(ROOT)):
        d = os.path.join(ROOT, name)
        notes = os.path.join(d, "notes.ja.md")
        if not os.path.isdir(d) or not os.path.isfile(notes):
            continue
        tid = topic_id_of(name)
        if tid is None:
            continue                          # index-*.csv 等の非スレ物は無視
        slug = name[len(tid):].lstrip("-")
        title, candidate = parse_notes(notes)
        rows.append({
            "topic_id": tid,
            "slug": slug,
            "title": title,
            "candidate": candidate,
            "url": "https://forums.atariage.com/topic/%s" % tid,
            "notes": "reference/atariage/%s/notes.ja.md" % name,
        })
    return rows


def parse_notes(path):
    """notes.ja.md 冒頭から見出し（タイトル）と技候補番号（あれば）を拾う。"""
    title = ""
    candidate = ""
    try:
        with open(path, encoding="utf-8") as f:
            head = f.readlines()[:30]
    except OSError:
        head = []
    for line in head:
        if not title and line.startswith("# "):
            title = line[2:].strip()
        m = re.search(r"技候補\s*([①-⑳⑦⑧])", line) or re.search(r"候補\s*([①-⑳])", line)
        if m and not candidate:
            candidate = m.group(1)
    return title, candidate


def write_csv(rows):
    out = os.path.join(ROOT, "MINED.csv")
    with open(out, "w", newline="", encoding="utf-8") as f:
        w = csv.DictWriter(f, fieldnames=["topic_id", "slug", "title", "candidate", "url", "notes"])
        w.writeheader()
        for r in rows:
            w.writerow(r)
    return out


def main():
    rows = scan()
    if len(sys.argv) >= 3 and sys.argv[1] == "--check":
        tid = topic_id_of(sys.argv[2])
        if tid is None:
            print("?? could not parse topic_id from: %s" % sys.argv[2])
            sys.exit(2)
        mined = {r["topic_id"]: r for r in rows}
        if tid in mined:
            print("MINED %s — %s (%s)" % (tid, mined[tid]["title"], mined[tid]["notes"]))
            sys.exit(1)
        print("NEW %s — not yet mined" % tid)
        sys.exit(0)
    out = write_csv(rows)
    print("manifest: %d mined threads -> %s" % (len(rows), out))


if __name__ == "__main__":
    main()
