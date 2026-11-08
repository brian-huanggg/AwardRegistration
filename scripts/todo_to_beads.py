#!/usr/bin/env python3
"""Convert docs/TODO.md (PR split plan) into beads.

Mapping
  ### Mx — name table section  -> one epic per milestone
  table row                    -> one task, child of its milestone epic
  Deps column                  -> `bd dep add` (blocked-by) edges
  §7.1 priority table          -> bd priority (P0=0, P1=1, P2a-h=2, P3=3)
  ID prefix letter / Size      -> labels (backend|frontend|devops|docs|test, size:S|M)
  Verify column                -> --acceptance
Each issue carries external-ref `plan:<ID>`, so re-running skips existing ones.
Rows with combined IDs (e.g. `B1-11/12`) become one issue; every alias resolves to it.

Usage:  python3 scripts/todo_to_beads.py [--file docs/TODO.md] [--dry-run]
"""
import argparse
import json
import re
import subprocess
import sys
from pathlib import Path

ID_RE = re.compile(r"\b[DBFOQ]\d*[a-z]?-\d+\b")
LABELS = {"D": "docs", "B": "backend", "F": "frontend", "O": "devops", "Q": "test"}
PRIORITY_ROWS = {"P0": 0, "P1": 1, "P3": 3, "Wrap-up": 1}  # P2a..P2h -> 2
MILESTONE_RE = re.compile(r"^###\s+(M\d+)\s+—\s+(.+)$")


def parse_ids(cell):
    """'B1b-02/03' -> ['B1b-02','B1b-03']; 'B0-04/O0-06' -> both."""
    ids, prefix = [], ""
    for part in cell.replace("`", "").split("/"):
        part = part.strip()
        if re.fullmatch(r"\d+", part):
            ids.append(f"{prefix}-{part}")
        else:
            ids.append(part)
            prefix = part.rsplit("-", 1)[0]
    return ids


def expand(token):
    """'B1-01–11' -> B1-01..B1-11; plain IDs pass through."""
    m = re.fullmatch(r"([A-Z]\d*[a-z]?)-(\d+)[–-](\d+)", token)
    if m:
        p, a, b = m.group(1), int(m.group(2)), int(m.group(3))
        return [f"{p}-{n:02d}" for n in range(a, b + 1)]
    return [token] if ID_RE.fullmatch(token) else []


def parse_priorities(lines):
    prio = {}
    in_table = False
    for line in lines:
        if line.startswith("| Pri |"):
            in_table = True
            continue
        if in_table:
            if not line.startswith("|"):
                break
            cells = [c.strip() for c in line.strip("|").split("|")]
            if len(cells) < 3 or set(cells[0]) <= {"-"}:
                continue
            key = cells[0]
            value = 2 if re.fullmatch(r"P2[a-z]", key) else PRIORITY_ROWS.get(key)
            if value is None:
                continue
            for tok in re.split(r"[,\s]+", cells[2]):
                for i in expand(tok.strip()):
                    prio.setdefault(i, value)
    return prio


def parse_milestones(lines):
    milestones, current = [], None
    for line in lines:
        m = MILESTONE_RE.match(line)
        if m:
            current = {"key": m.group(1), "name": m.group(2).strip(), "rows": []}
            milestones.append(current)
            continue
        if line.startswith("#"):
            current = None
        if current is None or not line.startswith("|"):
            continue
        cells = [c.strip() for c in re.split(r"(?<!\\)\|", line.strip().strip("|"))]
        if len(cells) != 5 or cells[0] in ("ID",) or set(cells[0]) <= {"-"}:
            continue
        ids = parse_ids(cells[0])
        current["rows"].append(
            {"ids": ids, "scope": cells[1], "verify": cells[2], "deps": cells[3], "size": cells[4]}
        )
    return [m for m in milestones if m["rows"]]


def short_title(ids, scope, limit=80):
    text = re.sub(r"[`*]", "", scope)
    text = re.split(r"(?<=[.;])\s|\s—\s", text, maxsplit=1)[0].rstrip(".;")
    if len(text) > limit:
        text = text[:limit].rsplit(" ", 1)[0] + "…"
    return f"{'/'.join(ids)}: {text}"


def bd(args, dry, capture=True, stdin=None):
    if dry:
        print("  $ bd " + " ".join(args))
        return "dry-" + args[-1] if args[0] == "create" else "dry-run"
    r = subprocess.run(["bd", *args], capture_output=capture, text=True, input=stdin)
    if r.returncode != 0:
        sys.exit(f"bd {' '.join(args)} failed:\n{r.stderr or r.stdout}")
    return r.stdout.strip()


def existing(ref):
    out = subprocess.run(
        ["bd", "list", "--all", "--external-ref", ref, "--json", "-n", "0"],
        capture_output=True, text=True,
    ).stdout.strip()
    try:
        data = json.loads(out) if out else []
    except json.JSONDecodeError:
        return None
    return data[0]["id"] if data else None


def create(title, ref, dry, **kw):
    if not dry and (found := existing(ref)):
        print(f"= {ref} exists ({found})")
        return found
    args = ["create", title, "--silent", "--external-ref", ref]
    for flag, val in kw.items():
        if val not in (None, "", []):
            args += [f"--{flag.replace('_', '-')}", ",".join(val) if isinstance(val, list) else str(val)]
    new_id = bd(args, dry)
    print(f"+ {ref} -> {new_id}")
    return new_id


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--file", default="docs/TODO.md")
    ap.add_argument("--dry-run", action="store_true", help="print bd commands, change nothing")
    a = ap.parse_args()

    lines = Path(a.file).read_text(encoding="utf-8").splitlines()
    prio = parse_priorities(lines)
    milestones = parse_milestones(lines)

    alias = {}  # plan ID -> bead ID
    edges = []  # (bead, depends-on plan ID)
    for ms in milestones:
        epic = create(f"{ms['key']} — {ms['name']}", f"plan:{ms['key']}", a.dry_run,
                      type="epic", priority=2, labels=["milestone"])
        for row in ms["rows"]:
            ids = row["ids"]
            p = next((prio[i] for i in ids if i in prio), 0 if ids[0].startswith("D-") else 2)
            labels = sorted({LABELS[i[0]] for i in ids}) + [f"size:{row['size']}"] + (
                ["blocked-by-OI"] if "OI-" in row["deps"] else [])
            bead = create(
                short_title(ids, row["scope"]), f"plan:{ids[0]}", a.dry_run,
                parent=epic, priority=p, labels=labels, type="task",
                description=f"**{'/'.join(ids)}** — {row['scope']}",
                acceptance=row["verify"],
                notes=f"Deps (source): {row['deps']}" if row["deps"] not in ("–", "-") else None,
            )
            for i in ids:
                alias[i] = bead
            for dep in sorted(set(ID_RE.findall(row["deps"])) - set(ids)):
                edges.append((bead, dep, ids[0]))

    # second pass: wire dependencies (unresolved refs like "M4–M8" / "all F PRs" stay in notes)
    batch, skipped = [], []
    for bead, dep, src in edges:
        if dep in alias:
            if alias[dep] != bead:
                batch.append({"from": bead, "to": alias[dep], "type": "blocks"})
        else:
            skipped.append(f"{src} -> {dep}")
    print(f"\n{len(batch)} dependency edges, {len(skipped)} unresolved: {', '.join(skipped) or 'none'}")
    if batch:
        payload = "\n".join(json.dumps(e) for e in batch)
        if a.dry_run:
            print("  $ bd dep add --file -   # " + str(len(batch)) + " edges")
        else:
            bd(["dep", "add", "--file", "-"], False, stdin=payload)
    print("Done." if not a.dry_run else "Dry run only.")


if __name__ == "__main__":
    main()
