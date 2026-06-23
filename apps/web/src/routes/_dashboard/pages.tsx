import { createFileRoute, Link } from "@tanstack/react-router";
import { Filter, Globe, Loader2, Search } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { PageHeader } from "@/components/page-header";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Separator } from "@/components/ui/separator";
import { useAllPages } from "@/features/pages";
import type { Page, PageProvider } from "@/features/pages/types";
import { useProjectNameMap } from "@/features/projects";
import { statusVariant } from "@/lib/constants";

const PAGE_PROVIDER_LABELS: Record<PageProvider, string> = {
  aws_cloudfront: "AWS CloudFront",
  cloudflare_pages: "Cloudflare Pages",
};

export const Route = createFileRoute("/_dashboard/pages")({
  component: PagesPage,
});

function PagesPage() {
  const [page, setPage] = useState(1);
  const [search, setSearch] = useState("");
  const [debouncedSearch, setDebouncedSearch] = useState("");
  const [statusFilter, setStatusFilter] = useState("");
  const [providerFilter, setProviderFilter] = useState<PageProvider | "">("");
  const { data: projectMap } = useProjectNameMap();
  const { data, isLoading } = useAllPages(
    page,
    20,
    debouncedSearch || undefined,
    statusFilter || undefined,
    providerFilter || undefined,
  );

  const pages = data?.items ?? [];
  const pagination = data?.pagination;
  const totalPages = pagination ? Math.ceil(pagination.total / pagination.per_page) : 1;

  const debounceRef = useRef<ReturnType<typeof setTimeout>>();
  useEffect(
    () => () => {
      clearTimeout(debounceRef.current);
    },
    [],
  );
  const handleSearch = (value: string) => {
    setSearch(value);
    setPage(1);
    clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(() => setDebouncedSearch(value), 300);
  };

  return (
    <div>
      <PageHeader
        title="Pages"
        description="Static sites deployed to AWS CloudFront or Cloudflare Pages."
      />
      <Separator className="my-5" />

      <div className="mb-4 flex items-center gap-3">
        <div className="relative flex-1 max-w-xs">
          <Search className="absolute left-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-muted-foreground/50" />
          <Input
            value={search}
            onChange={(e) => handleSearch(e.target.value)}
            placeholder="Search pages..."
            className="h-8 pl-8 text-sm"
          />
        </div>
        <div className="flex items-center gap-2">
          <Filter className="h-3.5 w-3.5 text-muted-foreground/50" />
          <Select
            value={providerFilter || "all"}
            onValueChange={(v) => {
              setProviderFilter(v === "all" ? "" : (v as PageProvider));
              setPage(1);
            }}
          >
            <SelectTrigger className="h-8 w-44 text-sm">
              <SelectValue placeholder="All providers" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">All providers</SelectItem>
              {(Object.keys(PAGE_PROVIDER_LABELS) as PageProvider[]).map((provider) => (
                <SelectItem key={provider} value={provider}>
                  {PAGE_PROVIDER_LABELS[provider]}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Select
            value={statusFilter || "all"}
            onValueChange={(v) => {
              setStatusFilter(v === "all" ? "" : v);
              setPage(1);
            }}
          >
            <SelectTrigger className="h-8 w-36 text-sm">
              <SelectValue placeholder="All statuses" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">All statuses</SelectItem>
              <SelectItem value="live">Live</SelectItem>
              <SelectItem value="deploying">Deploying</SelectItem>
              <SelectItem value="idle">Idle</SelectItem>
              <SelectItem value="error">Error</SelectItem>
              <SelectItem value="draining">Draining</SelectItem>
            </SelectContent>
          </Select>
        </div>
      </div>

      {isLoading ? (
        <div className="flex justify-center py-12">
          <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
        </div>
      ) : pages.length === 0 ? (
        <Card>
          <CardContent className="flex flex-col items-center justify-center py-10 text-muted-foreground">
            <div className="flex h-10 w-10 items-center justify-center rounded-full bg-primary/10">
              <Globe className="h-5 w-5 text-primary" />
            </div>
            <p className="mt-3 text-sm text-muted-foreground">
              {debouncedSearch || statusFilter || providerFilter
                ? "No pages match your filters."
                : "No pages yet."}
            </p>
          </CardContent>
        </Card>
      ) : (
        <>
          <div className="space-y-2">
            {pages.map((item) => (
              <PageRow key={item.id} page={item} projectName={projectMap?.get(item.project_id)} />
            ))}
          </div>

          {pagination && totalPages > 1 && (
            <div className="mt-4 flex items-center justify-between">
              <p className="text-xs text-muted-foreground">
                Page {pagination.page} of {totalPages} ({pagination.total} total)
              </p>
              <div className="flex gap-2">
                <Button
                  size="sm"
                  variant="outline"
                  disabled={page <= 1}
                  onClick={() => setPage(page - 1)}
                >
                  Previous
                </Button>
                <Button
                  size="sm"
                  variant="outline"
                  disabled={page >= totalPages}
                  onClick={() => setPage(page + 1)}
                >
                  Next
                </Button>
              </div>
            </div>
          )}
        </>
      )}
    </div>
  );
}

function PageRow({ page, projectName }: { page: Page; projectName?: string }) {
  return (
    <Link
      to="/projects/$id/pages/$pageId"
      params={{ id: page.project_id, pageId: page.id }}
      className="block"
    >
      <Card className="transition-colors hover:bg-accent/50">
        <CardContent className="flex items-center gap-3 p-4">
          <Globe className="h-4 w-4 shrink-0 text-primary" />
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-2">
              <span className="truncate text-sm font-medium">{page.name}</span>
              {projectName && (
                <Badge
                  variant="outline"
                  className="shrink-0 text-xs font-normal text-muted-foreground"
                >
                  {projectName}
                </Badge>
              )}
            </div>
            <p className="mt-0.5 truncate text-xs text-muted-foreground">
              {PAGE_PROVIDER_LABELS[page.provider]} · {page.git_repo} @ {page.git_branch}
            </p>
          </div>
          <Badge variant={statusVariant(page.status)} className="shrink-0 text-xs">
            {page.status}
          </Badge>
        </CardContent>
      </Card>
    </Link>
  );
}
