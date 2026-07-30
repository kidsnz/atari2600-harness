#!/usr/bin/env python3
"""check_memory.py — 永続メモリ（~/.claude/.../memory/）の構造検査。

なぜ要るか（2026-07-30 実測）: `docs/` には check_wiring / check_provenance があり、両方とも
実際に穴を見つけた。**memory には検査が1つも無かった。** 38 本・1,786 行が、誰にも確かめられない
まま毎セッション読み込まれていた。整理（統合・書き直し）はここから始めるべきで、ゲートの無い
まま書き直すのは、このプロジェクトが一晩かけて潰していた「網の外にあるものが壊れる」そのもの。

検査するもの:
  ① [[wiki-link]] が実在する memory **または** harness/docs のファイルを指しているか
  ② 全ファイルが MEMORY.md に **ちょうど1行** あり、索引の行も実在ファイルを指しているか
  ③ frontmatter が揃っているか（name / description / metadata.type）かつ name == ファイル名
  ④ 1ファイルの行数上限（再肥大の防止）
  ⑥ harness/docs と CLAUDE.md からの [[link]] が現役の memory を指しているか
     — 2026-07-30 実測: memory を1本 _archive/ に移した直後、docs/authoring-protocol.md:5 の
       [[feedback-authoring-loop-system]] が宙に浮いた。memory→docs だけ見ていて逆向きを
       見ていなかったので、最初の版はこれを見逃した。片方向の検査は検査の半分。
  ⑤ ★正本ルールが「事件」を保っているか＝具体物（ファイル名・テスト名・数値）を一定数引いているか
     — ルールが噛むのは抽象論ではなく具体例。整理すると見出しだけ残って事例が落ちる。それを機械で止める。

使い方:
    cd harness && python3 scripts/check_memory.py
    MEMORY_DIR=/path/to/memory python3 scripts/check_memory.py
memory ディレクトリが無い環境では skip（exit 0）。
"""
import os
import re
import sys

DEFAULT = os.path.expanduser(
    "~/.claude/projects/-Users-shinji-Documents-2D-260609-atari2600-dev/memory")
MEM = os.environ.get("MEMORY_DIR", DEFAULT)

# 1ファイルの上限。実測(2026-07-30)の最大は project-next-session-todo.md=321、
# 次が feedback-verification-standard.md=197。上限はその間ではなく「正本として読める長さ」で引く。
MAX_LINES = 250

# 上限の明示的な例外。**理由を書かないと通らない**（黙って緩めない）。
# 借金は隠すのでなく grep できる形にする — roms 側の `@rom-write-ok` と同じ流儀。
OVERSIZE_EXEMPT = {
    "project-next-session-todo.md":
        "322 lines / description: 8,361 chars on ONE line (2026-07-30 measured). The body holds 28 "
        "dated handoffs back to 2026-06-17. It is NOT redundant: of 4 sampled sections, 2 "
        "(PONG capstone 完成 / Combat v0.8.0) appear NOWHERE in STATUS.md, so trimming on the "
        "assumption that STATUS.md is the canonical board would delete the only copy. The fix is to "
        "move the history INTO STATUS.md first (completing the board), then cut here — real work, "
        "not a line-count edit. Exempted with this reason rather than raising the cap for everyone.",
}

# 正本ルール。ここは中身を薄くしてはいけない側。
CANONICAL = [
    "feedback-verification-standard.md",
    "feedback-goal-standard.md",
    "feedback-execution-discipline.md",
    "feedback-work-tracking.md",
]
MIN_CONCRETE = 3  # 正本が引くべき具体物の最少数

# 「具体物」= リポジトリの実在物やテスト名や測定値。抽象論だけの正本を落とすための指標。
CONCRETE = re.compile(
    r"`[^`]*\.(?:asm|go|py|json|md|bin)`"      # ファイル名
    r"|`?Test[A-Z][A-Za-z0-9_]+`?"              # Go テスト名
    r"|\b[0-9a-f]{7}\b"                         # コミットハッシュ
    r"|\b\d+\s*(?:件|本|行|回|cy|サイクル|px)\b"  # 単位つきの実測値
)
LINK = re.compile(r"\[\[([^\]]+)\]\]")
HARNESS_DOCS = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "docs")


def INDEX_ENTRY(fname):
    """索引の1項目 = 行頭の `- [Title](file.md)`。本文中の参照とは形が違う。"""
    return re.compile(r"^- \[[^\]]*\]\(" + re.escape(fname) + r"\)", re.M)

FM_NAME = re.compile(r"^name:\s*(\S+)\s*$", re.M)
FM_DESC = re.compile(r"^description:\s*\S", re.M)
FM_TYPE = re.compile(r"^\s*type:\s*(user|feedback|project|reference)\s*$", re.M)


def main():
    if not os.path.isdir(MEM):
        print(f"memory not present ({MEM}) — skipped")
        return 0
    index_path = os.path.join(MEM, "MEMORY.md")
    files = sorted(f for f in os.listdir(MEM)
                   if f.endswith(".md") and f != "MEMORY.md")
    if not files:
        print("no memory files — skipped")
        return 0

    errors = []
    index = open(index_path, encoding="utf-8").read() if os.path.isfile(index_path) else ""
    if not index:
        errors.append("MEMORY.md is missing or empty — the index is what gets loaded every session")

    stems = {f[:-3] for f in files}
    concrete_counts = {}

    for f in files:
        path = os.path.join(MEM, f)
        text = open(path, encoding="utf-8").read()
        lines = text.count("\n") + 1

        # ③ frontmatter
        m = FM_NAME.search(text)
        if not m:
            errors.append(f"{f}: no `name:` in frontmatter")
        elif m.group(1) != f[:-3]:
            errors.append(f"{f}: frontmatter name `{m.group(1)}` does not match the filename")
        if not FM_DESC.search(text):
            errors.append(f"{f}: no `description:` — recall picks files by this line")
        if not FM_TYPE.search(text):
            errors.append(f"{f}: no `metadata.type:` of user/feedback/project/reference")

        # ④ 上限
        if lines > MAX_LINES and f not in OVERSIZE_EXEMPT:
            errors.append(f"{f}: {lines} lines, over the {MAX_LINES} cap — split it rather than let "
                          f"one file accrete (one fact per file). If it must stay, add it to "
                          f"OVERSIZE_EXEMPT WITH A MEASURED REASON.")
        if f in OVERSIZE_EXEMPT and lines <= MAX_LINES:
            errors.append(f"{f}: is exempt from the {MAX_LINES}-line cap but is now {lines} lines — "
                          f"the debt was paid, so drop the exemption rather than leave a licence "
                          f"nobody needs")

        # ① リンク。memory 同士だけでなく harness/docs も正当な行き先なので、
        # 禁じるのではなく「どちらかに実在するか」を検査する（実測: feedback-verification-standard
        # は [[known-traps]] で docs/known-traps.md を指していた）。
        for target in LINK.findall(text):
            if target in stems:
                continue
            if any(os.path.isfile(os.path.join(HARNESS_DOCS, sub, target + ".md"))
                   for sub in ("", "techniques")):
                continue
            errors.append(f"{f}: [[{target}]] resolves to neither a memory nor a harness doc")

        # ② 索引。数えるのは **索引項目の行** だけ。本文中の相互参照
        # （「正本＝[x](x.md)」など）は正当な書き方なので数に入れない。
        n = len(INDEX_ENTRY(f).findall(index))
        if n == 0:
            errors.append(f"{f}: absent from MEMORY.md — a memory nobody indexes is not recalled")
        elif n > 1:
            errors.append(f"{f}: appears {n} times in MEMORY.md; the index holds one line per memory")

        concrete_counts[f] = len(set(CONCRETE.findall(text)))

    # ⑥ 逆向き: harness の docs / CLAUDE.md から memory を指すリンク
    repo = os.path.normpath(os.path.join(os.path.dirname(os.path.abspath(__file__)), ".."))
    sources = [os.path.join(repo, "CLAUDE.md")]
    for base, _, fs in os.walk(os.path.join(repo, "docs")):
        sources += [os.path.join(base, x) for x in fs if x.endswith(".md")]
    doc_stems = set()
    for base, _, fs in os.walk(os.path.join(repo, "docs")):
        doc_stems |= {x[:-3] for x in fs if x.endswith(".md")}
    for src in sources:
        if not os.path.isfile(src):
            continue
        try:
            body = open(src, encoding="utf-8").read()
        except OSError:
            continue
        for target in set(LINK.findall(body)):
            if target in stems or target in doc_stems:
                continue
            rel = os.path.relpath(src, repo)
            arch = os.path.isfile(os.path.join(MEM, "_archive", target + ".md"))
            why = "it was archived" if arch else "no such memory or doc"
            errors.append(f"{rel}: [[{target}]] resolves to nothing ({why})")

    # 索引が実在しないファイルを指していないか
    for ref in re.findall(r"\(([a-z0-9-]+\.md)\)", index):
        if ref not in files:
            errors.append(f"MEMORY.md links to {ref}, which does not exist")

    # ⑤ 正本の具体性
    for c in CANONICAL:
        if c not in files:
            errors.append(f"canonical rule file {c} is missing")
            continue
        if concrete_counts[c] < MIN_CONCRETE:
            errors.append(
                f"{c}: only {concrete_counts[c]} concrete citations (files, test names, commit "
                f"hashes, measured values); a rule bites because of the incident behind it, and a "
                f"tidy-up that keeps the heading and drops the evidence silently stops it working")

    if errors:
        print("MEMORY CHECK FAILED:")
        for e in errors:
            print("  ✗", e)
        return 1

    total = sum(open(os.path.join(MEM, f), encoding="utf-8").read().count("\n") + 1 for f in files)
    if OVERSIZE_EXEMPT:
        print(f"note: {len(OVERSIZE_EXEMPT)} file(s) exempt from the {MAX_LINES}-line cap: "
              f"{', '.join(sorted(OVERSIZE_EXEMPT))}")
    print(f"memory OK — {len(files)} files / {total} lines, index complete, all [[links]] resolve, "
          f"{len(CANONICAL)} canonical rules each citing >= {MIN_CONCRETE} concrete artefacts "
          f"({', '.join(f'{c.split(chr(46))[0][:22]}:{concrete_counts[c]}' for c in CANONICAL)})")
    return 0


if __name__ == "__main__":
    sys.exit(main())
