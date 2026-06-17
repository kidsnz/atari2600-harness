import os, re, sys, urllib.request, csv
from curl_cffi import requests
UA="Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Safari/537.36"
ck=open("../reference/atariage/aa_cookie.txt").read().strip()

def fetch(url, wb_ts):
    try:
        r=requests.get(url, headers={"Cookie":ck,"User-Agent":UA}, impersonate="chrome", timeout=25)
        if r.status_code==200 and "just a moment" not in r.text.lower(): return r.text
    except Exception: pass
    try:
        return urllib.request.urlopen(urllib.request.Request(f"http://web.archive.org/web/{wb_ts}id_/{url}", headers={"User-Agent":UA}), timeout=40).read().decode("utf-8","ignore")
    except Exception:
        return ""

def text(html):
    blocks=re.findall(r'ipsType_richText.*?>(.*?)</section>', html, re.S) or re.findall(r'ipsType_richText.*?>(.*?)</div>', html, re.S)
    t=" ".join(re.sub(r'<[^>]+>',' ',b) for b in blocks)
    t=re.sub(r'&nbsp;',' ',t); t=re.sub(r'&amp;','&',t); t=re.sub(r'&lt;','<',t); t=re.sub(r'&gt;','>',t); t=re.sub(r'&quot;','"',t); t=re.sub(r'&#39;',"'",t)
    return re.sub(r'\s+',' ', t).strip()

def comments(html):
    # grab comment bodies if present
    cb=re.findall(r'data-role="commentContent".*?>(.*?)</div>', html, re.S)
    t=" ".join(re.sub(r'<[^>]+>',' ',b) for b in cb)
    return re.sub(r'\s+',' ', t).strip()[:4000]

rows=list(csv.DictReader(open("../reference/atariage/blog_slices/blog-2.csv")))
outdir="_blogtmp"
os.makedirs(outdir, exist_ok=True)
start=int(sys.argv[1]) if len(sys.argv)>1 else 0
end=int(sys.argv[2]) if len(sys.argv)>2 else len(rows)
for r in rows[start:end]:
    eid=r["entry_id"]; slug=r["slug"]
    notes=f"../reference/atariage/blogs/{eid}-{slug}/notes.ja.md"
    if os.path.exists(notes):
        print(f"SKIP {eid}"); continue
    html=fetch(r["url"], r["wb_ts"])
    body=text(html)
    if len(body)<200:
        # looser
        m=re.findall(r'<p>(.*?)</p>', html, re.S)
        body=re.sub(r'\s+',' '," ".join(re.sub(r'<[^>]+>',' ',x) for x in m)).strip()
    cm=comments(html)
    full=body+("\n\n[COMMENTS]\n"+cm if cm else "")
    open(f"{outdir}/{eid}.txt","w").write(full)
    print(f"FETCH {eid} {slug} len={len(body)} cm={len(cm)}")
