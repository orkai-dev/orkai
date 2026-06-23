import { Check, ChevronsUpDown, Loader2, Search } from "lucide-react";
import { useEffect, useMemo, useRef, useState } from "react";
import { cn } from "@/lib/utils";

const MAX_RENDERED = 100;

function filterBuckets(buckets: string[], query: string): string[] {
  const q = query.trim().toLowerCase();
  if (!q) return buckets;
  return buckets
    .filter((b) => b.toLowerCase().includes(q))
    .sort((a, b) => {
      const al = a.toLowerCase();
      const bl = b.toLowerCase();
      const ai = al.startsWith(q) ? 0 : 1;
      const bi = bl.startsWith(q) ? 0 : 1;
      return ai - bi || al.localeCompare(bl);
    });
}

export function BucketCombobox({
  buckets,
  value,
  onSelect,
  loading,
  disabled,
  error,
  placeholder = "Search buckets...",
}: {
  buckets: string[];
  value: string;
  onSelect: (bucket: string) => void;
  loading?: boolean;
  disabled?: boolean;
  error?: string | null;
  placeholder?: string;
}) {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [active, setActive] = useState(0);
  const containerRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const listRef = useRef<HTMLDivElement>(null);

  const filtered = useMemo(() => filterBuckets(buckets, query), [buckets, query]);
  const visible = filtered.slice(0, MAX_RENDERED);

  // A typed name that isn't an existing bucket can still be used (e.g. when the
  // account lacks s3:ListAllMyBuckets but can write to a known bucket).
  const trimmed = query.trim();
  const showCustom = trimmed.length > 0 && !buckets.some((b) => b === trimmed);

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

  useEffect(() => {
    if (open) {
      setQuery("");
      setActive(0);
      const t = setTimeout(() => inputRef.current?.focus(), 0);
      return () => clearTimeout(t);
    }
  }, [open]);

  useEffect(() => {
    const el = listRef.current?.querySelector<HTMLElement>(`[data-idx="${active}"]`);
    el?.scrollIntoView({ block: "nearest" });
  }, [active]);

  function choose(bucket: string) {
    onSelect(bucket);
    setOpen(false);
  }

  function onKeyDown(e: React.KeyboardEvent) {
    const options = showCustom ? visible.length + 1 : visible.length;
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setActive((a) => Math.min(a + 1, options - 1));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setActive((a) => Math.max(a - 1, 0));
    } else if (e.key === "Enter") {
      e.preventDefault();
      if (showCustom && active === visible.length) {
        choose(trimmed);
      } else if (visible[active]) {
        choose(visible[active]);
      }
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
        <span className={cn("flex-1 truncate text-left", !value && "text-muted-foreground")}>
          {value || "Select a bucket..."}
        </span>
        {loading ? (
          <Loader2 className="h-4 w-4 shrink-0 animate-spin opacity-50" />
        ) : (
          <ChevronsUpDown className="h-4 w-4 shrink-0 opacity-50" />
        )}
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
            {loading ? (
              <p className="flex items-center justify-center gap-2 px-3 py-6 text-center text-sm text-muted-foreground">
                <Loader2 className="h-4 w-4 animate-spin" /> Loading buckets...
              </p>
            ) : error ? (
              <p className="px-3 py-6 text-center text-sm text-destructive">{error}</p>
            ) : (
              <>
                {visible.map((bucket, i) => (
                  <button
                    key={bucket}
                    type="button"
                    data-idx={i}
                    onMouseEnter={() => setActive(i)}
                    onClick={() => choose(bucket)}
                    className={cn(
                      "flex w-full items-center gap-2 px-2 py-1.5 text-left text-sm",
                      i === active ? "bg-accent text-accent-foreground" : "text-foreground",
                    )}
                  >
                    <Check
                      className={cn(
                        "h-4 w-4 shrink-0",
                        bucket === value ? "opacity-100 text-primary" : "opacity-0",
                      )}
                    />
                    <span className="flex-1 truncate">{bucket}</span>
                  </button>
                ))}
                {showCustom && (
                  <button
                    type="button"
                    data-idx={visible.length}
                    onMouseEnter={() => setActive(visible.length)}
                    onClick={() => choose(trimmed)}
                    className={cn(
                      "flex w-full items-center gap-2 px-2 py-1.5 text-left text-sm",
                      active === visible.length
                        ? "bg-accent text-accent-foreground"
                        : "text-foreground",
                    )}
                  >
                    <Check className="h-4 w-4 shrink-0 opacity-0" />
                    <span className="flex-1 truncate">
                      Use “<span className="font-medium">{trimmed}</span>”
                    </span>
                  </button>
                )}
                {!showCustom && visible.length === 0 && (
                  <p className="px-3 py-6 text-center text-sm text-muted-foreground">
                    {buckets.length === 0 ? "No buckets found." : `No buckets match “${query}”.`}
                  </p>
                )}
              </>
            )}
          </div>

          {!loading && !error && filtered.length > MAX_RENDERED && (
            <div className="border-t px-3 py-1.5 text-center text-xs text-muted-foreground">
              Showing {MAX_RENDERED} of {filtered.length} — keep typing to narrow results
            </div>
          )}
        </div>
      )}
    </div>
  );
}
