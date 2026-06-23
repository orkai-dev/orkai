import { Check, ChevronsUpDown, Lock, Search } from "lucide-react";
import { useEffect, useMemo, useRef, useState } from "react";
import type { GitRepo } from "@/features/resources";
import { cn } from "@/lib/utils";

// Cap how many results we render at once. Filtering happens over the full list,
// but the DOM stays light even with thousands of repos.
const MAX_RENDERED = 100;

interface ScoredRepo {
  repo: GitRepo;
  score: number;
  matchStart: number;
  matchEnd: number;
}

function scoreRepos(repos: GitRepo[], query: string): ScoredRepo[] {
  const q = query.trim().toLowerCase();
  if (!q) {
    return repos.map((repo) => ({ repo, score: 3, matchStart: -1, matchEnd: -1 }));
  }

  const results: ScoredRepo[] = [];
  for (const repo of repos) {
    const full = repo.full_name.toLowerCase();
    const short = repo.name.toLowerCase();
    const idx = full.indexOf(q);
    if (idx === -1) continue;

    // Rank: exact short name > short name prefix > full name prefix > substring.
    let score = 3;
    if (short === q) score = 0;
    else if (short.startsWith(q)) score = 1;
    else if (full.startsWith(q)) score = 2;

    results.push({ repo, score, matchStart: idx, matchEnd: idx + q.length });
  }

  results.sort(
    (a, b) =>
      a.score - b.score ||
      a.matchStart - b.matchStart ||
      a.repo.full_name.localeCompare(b.repo.full_name),
  );
  return results;
}

function Highlight({ text, start, end }: { text: string; start: number; end: number }) {
  if (start < 0 || end <= start) return <>{text}</>;
  return (
    <>
      {text.slice(0, start)}
      <mark className="bg-primary/20 text-foreground">{text.slice(start, end)}</mark>
      {text.slice(end)}
    </>
  );
}

export function RepoCombobox({
  repos,
  value,
  onSelect,
  disabled,
  placeholder = "Search repositories...",
}: {
  repos: GitRepo[];
  /** Selected repo's full_name. */
  value: string;
  onSelect: (repo: GitRepo) => void;
  disabled?: boolean;
  placeholder?: string;
}) {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [active, setActive] = useState(0);
  const containerRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const listRef = useRef<HTMLDivElement>(null);

  const selected = repos.find((r) => r.full_name === value);

  const scored = useMemo(() => scoreRepos(repos, query), [repos, query]);
  const visible = scored.slice(0, MAX_RENDERED);

  // Close on outside click.
  useEffect(() => {
    if (!open) return;
    function onDocClick(e: MouseEvent) {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    }
    document.addEventListener("mousedown", onDocClick);
    return () => document.removeEventListener("mousedown", onDocClick);
  }, [open]);

  // Focus the search input and reset state when opening.
  useEffect(() => {
    if (open) {
      setQuery("");
      setActive(0);
      const t = setTimeout(() => inputRef.current?.focus(), 0);
      return () => clearTimeout(t);
    }
  }, [open]);

  // Keep the active option scrolled into view.
  useEffect(() => {
    const el = listRef.current?.querySelector<HTMLElement>(`[data-idx="${active}"]`);
    el?.scrollIntoView({ block: "nearest" });
  }, [active]);

  function choose(idx: number) {
    const item = visible[idx]?.repo;
    if (!item) return;
    onSelect(item);
    setOpen(false);
  }

  function onKeyDown(e: React.KeyboardEvent) {
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setActive((a) => Math.min(a + 1, visible.length - 1));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setActive((a) => Math.max(a - 1, 0));
    } else if (e.key === "Enter") {
      e.preventDefault();
      choose(active);
    } else if (e.key === "Escape") {
      e.preventDefault();
      setOpen(false);
    }
  }

  return (
    <div ref={containerRef} className="relative">
      <button
        type="button"
        disabled={disabled}
        onClick={() => setOpen((v) => !v)}
        className={cn(
          "flex h-8 w-full items-center gap-2 border border-border bg-background px-3 py-2 text-sm focus:outline-none focus:border-ring disabled:cursor-not-allowed disabled:opacity-50",
        )}
      >
        <span className={cn("flex-1 truncate text-left", !selected && "text-muted-foreground")}>
          {selected ? (
            <span className="flex items-center gap-1.5">
              <span className="truncate">{selected.full_name}</span>
              {selected.private && <Lock className="h-3 w-3 shrink-0 text-muted-foreground" />}
            </span>
          ) : (
            "Select a repository..."
          )}
        </span>
        <ChevronsUpDown className="h-4 w-4 shrink-0 opacity-50" />
      </button>

      {open && (
        <div className="absolute z-50 mt-1 w-full overflow-hidden border bg-popover text-popover-foreground">
          <div className="flex items-center gap-2 border-b px-3">
            <Search className="h-4 w-4 shrink-0 text-muted-foreground/60" />
            <input
              ref={inputRef}
              value={query}
              onChange={(e) => {
                setQuery(e.target.value);
                setActive(0);
              }}
              onKeyDown={onKeyDown}
              placeholder={placeholder}
              className="h-9 flex-1 bg-transparent text-sm outline-none placeholder:text-muted-foreground/50"
            />
          </div>

          <div ref={listRef} className="max-h-[280px] overflow-y-auto p-1">
            {visible.length === 0 ? (
              <p className="px-3 py-6 text-center text-sm text-muted-foreground">
                No repositories match “{query}”.
              </p>
            ) : (
              visible.map((s, i) => (
                <button
                  key={s.repo.full_name}
                  type="button"
                  data-idx={i}
                  onMouseEnter={() => setActive(i)}
                  onClick={() => choose(i)}
                  className={cn(
                    "flex w-full items-center gap-2 px-2 py-1.5 text-left text-sm",
                    i === active ? "bg-accent text-accent-foreground" : "text-foreground",
                  )}
                >
                  <Check
                    className={cn(
                      "h-4 w-4 shrink-0",
                      s.repo.full_name === value ? "opacity-100 text-primary" : "opacity-0",
                    )}
                  />
                  <span className="flex-1 truncate">
                    <Highlight text={s.repo.full_name} start={s.matchStart} end={s.matchEnd} />
                  </span>
                  {s.repo.private && <Lock className="h-3 w-3 shrink-0 text-muted-foreground/70" />}
                </button>
              ))
            )}
          </div>

          {scored.length > MAX_RENDERED && (
            <div className="border-t px-3 py-1.5 text-center text-xs text-muted-foreground">
              Showing {MAX_RENDERED} of {scored.length} — keep typing to narrow results
            </div>
          )}
        </div>
      )}
    </div>
  );
}
