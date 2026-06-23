import { Link } from "@tanstack/react-router";
import { Box, ChevronRight, Clock, Database, Globe, Rocket, Trash2, Zap } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { useDeploy } from "@/features/apps";
import { ENGINE_LABELS, statusVariant } from "@/lib/constants";
import type { ServiceItem } from "./-service-types";

function serviceRowMeta(svc: ServiceItem, projectId: string) {
  switch (svc.type) {
    case "app": {
      const item = svc.data;
      return {
        Icon: Box,
        typeBadge: "Application",
        subtitle:
          item.source_type === "image"
            ? item.docker_image
            : `${item.git_repo} @ ${item.git_branch}`,
        detailTo: "/projects/$id/apps/$appId" as const,
        detailParams: { id: projectId, appId: item.id },
        showDeploy: true,
      };
    }
    case "page": {
      const item = svc.data;
      return {
        Icon: Globe,
        typeBadge: "Page",
        subtitle: `${item.git_repo} @ ${item.git_branch}`,
        detailTo: "/projects/$id/pages/$pageId" as const,
        detailParams: { id: projectId, pageId: item.id },
        showDeploy: false,
      };
    }
    case "worker": {
      const item = svc.data;
      return {
        Icon: Zap,
        typeBadge: "Cloudflare Worker",
        subtitle: `${item.git_repo} @ ${item.git_branch}`,
        detailTo: "/projects/$id/workers/$workerId" as const,
        detailParams: { id: projectId, workerId: item.id },
        showDeploy: false,
      };
    }
    case "database": {
      const item = svc.data;
      return {
        Icon: Database,
        typeBadge: ENGINE_LABELS[item.engine] || "Database",
        subtitle: `v${item.version} · ${item.storage_size}`,
        detailTo: "/projects/$id/databases/$dbId" as const,
        detailParams: { id: projectId, dbId: item.id },
        showDeploy: false,
      };
    }
    case "cronjob": {
      const item = svc.data;
      const schedule = item.cron_expression;
      const target =
        item.source_type === "git" ? `${item.git_repo} @ ${item.git_branch}` : item.image;
      return {
        Icon: Clock,
        typeBadge: "CronJob",
        subtitle: `${schedule} · ${target || item.command}`,
        detailTo: "/cronjobs" as const,
        detailParams: undefined,
        showDeploy: false,
      };
    }
  }
}

export function ServiceList({
  services,
  projectId,
  onDelete,
}: {
  services: ServiceItem[];
  projectId: string;
  onDelete: (svc: ServiceItem) => void;
}) {
  return (
    <div className="space-y-2">
      {services.map((svc) => (
        <ServiceRow
          key={`${svc.type}-${svc.data.id}`}
          service={svc}
          projectId={projectId}
          onDelete={() => onDelete(svc)}
        />
      ))}
    </div>
  );
}

export function ServiceRow({
  service: svc,
  projectId,
  onDelete,
}: {
  service: ServiceItem;
  projectId: string;
  onDelete: () => void;
}) {
  const item = svc.data;
  const { Icon, typeBadge, subtitle, detailTo, detailParams, showDeploy } = serviceRowMeta(
    svc,
    projectId,
  );
  const deploy = useDeploy(svc.type === "app" ? item.id : "");

  return (
    <Card className="group transition-colors hover:bg-accent/50">
      <CardContent className="flex items-center gap-4 p-4">
        <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-primary/10">
          <Icon className="h-5 w-5 text-primary" />
        </div>
        <Link
          to={detailTo}
          {...(detailParams ? { params: detailParams } : {})}
          className="min-w-0 flex-1"
        >
          <div className="flex items-center gap-2">
            <span className="text-sm font-medium">{item.name}</span>
            <Badge variant="outline" className="text-xs">
              {typeBadge}
            </Badge>
            <Badge variant={statusVariant(item.status)} className="text-xs">
              {item.status}
            </Badge>
          </div>
          <p className="truncate text-xs text-muted-foreground">{subtitle}</p>
        </Link>
        <div className="flex shrink-0 items-center gap-1 opacity-0 transition-opacity group-hover:opacity-100">
          {showDeploy && (
            <Button
              size="icon"
              variant="ghost"
              className="h-8 w-8"
              onClick={() => deploy.mutate()}
              disabled={deploy.isPending}
            >
              <Rocket className="h-3.5 w-3.5" />
            </Button>
          )}
          <Button
            size="icon"
            variant="ghost"
            className="h-8 w-8 text-destructive hover:text-destructive"
            onClick={onDelete}
          >
            <Trash2 className="h-3.5 w-3.5" />
          </Button>
          <Link to={detailTo} {...(detailParams ? { params: detailParams } : {})}>
            <ChevronRight className="h-4 w-4 text-muted-foreground" />
          </Link>
        </div>
      </CardContent>
    </Card>
  );
}
