import { Box } from "lucide-react";
import { EmptyState } from "@/components/empty-state";
import { LoadingScreen } from "@/components/loading-screen";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent } from "@/components/ui/card";
import { useHelmReleases } from "@/features/cluster";
import type { HelmRelease } from "@/features/cluster/types";
import { timeAgo } from "@/lib/format";
import { ErrorBanner, helmStatusVariant } from "./-shared";

export function HelmTab() {
  const { data: releases, isLoading, isError } = useHelmReleases();

  if (isError) return <ErrorBanner message="Failed to load Helm releases" />;
  if (isLoading) return <LoadingScreen variant="detail" />;

  if (!releases || releases.length === 0) {
    return <EmptyState icon={Box} message="No Helm releases found" />;
  }

  return (
    <div className="mt-3 space-y-3">
      {releases.map((release: HelmRelease) => (
        <Card key={`${release.namespace}/${release.name}`}>
          <CardContent className="flex items-center gap-4 p-4">
            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-2">
                <span className="text-sm font-bold">{release.name}</span>
                <Badge variant="outline" className="text-xs">
                  {release.namespace}
                </Badge>
                <Badge variant={helmStatusVariant(release.status)} className="text-xs">
                  {release.status}
                </Badge>
              </div>
              <p className="text-xs text-muted-foreground">
                {release.chart} · revision {release.revision}
                {release.updated && ` · updated ${timeAgo(release.updated)}`}
              </p>
            </div>
          </CardContent>
        </Card>
      ))}
    </div>
  );
}
