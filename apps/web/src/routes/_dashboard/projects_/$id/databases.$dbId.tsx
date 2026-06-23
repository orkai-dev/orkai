import { createFileRoute } from "@tanstack/react-router";
import { Database, Loader2 } from "lucide-react";
import { useState } from "react";
import { LoadingScreen } from "@/components/loading-screen";
import { PageHeader } from "@/components/page-header";
import { Badge } from "@/components/ui/badge";
import { Separator } from "@/components/ui/separator";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  useDatabase,
  useDatabaseBackups,
  useDatabaseCredentials,
  useDatabasePods,
  useDatabaseStatus,
  useDeleteDatabase,
  useRestoreBackup,
  useTriggerBackup,
} from "@/features/databases";
import { ENGINE_LABELS, statusVariant } from "@/lib/constants";
import { DbBackupsSection } from "./_components/-db-backups-section";
import { DbConnectionSection } from "./_components/-db-connection-section";
import { DbDeleteDialog } from "./_components/-db-delete-dialog";
import { DbPodsSection } from "./_components/-db-pods-section";
import { DbSettingsSection } from "./_components/-db-settings-section";
import { DbWaitingPanel } from "./_components/-db-waiting-panel";

export const Route = createFileRoute("/_dashboard/projects_/$id/databases/$dbId")({
  component: DatabaseDetailPage,
});

function DatabaseDetailPage() {
  const { id: projectId, dbId } = Route.useParams();

  const { data: db, isLoading } = useDatabase(dbId);
  const { data: dbStatus } = useDatabaseStatus(dbId);
  const livePhaseEarly = dbStatus?.phase ?? db?.status;
  const { data: credentials, isLoading: credsLoading } = useDatabaseCredentials(
    dbId,
    livePhaseEarly === "running",
  );
  const { data: rawPods } = useDatabasePods(dbId);
  const { data: rawBackups } = useDatabaseBackups(dbId);
  const pods = rawPods ?? [];
  const backups = rawBackups ?? [];

  const deleteDb = useDeleteDatabase(dbId);
  const triggerBackup = useTriggerBackup(dbId);
  const restoreBackup = useRestoreBackup(dbId);

  const [showDelete, setShowDelete] = useState(false);
  const [confirmName, setConfirmName] = useState("");

  if (isLoading) return <LoadingScreen variant="detail" />;
  if (!db) return null;

  const livePhase = livePhaseEarly ?? db.status;
  const isReady = livePhase === "running";
  const isStarting = livePhase === "pending" || livePhase === "creating";

  return (
    <div>
      <PageHeader
        title={db.name}
        useBack
        description={
          <span className="flex items-center gap-1">
            <Database className="h-3 w-3" />
            {ENGINE_LABELS[db.engine]} v{db.version}
          </span>
        }
        badges={
          <>
            <Badge variant={statusVariant(livePhase)}>
              {isStarting && <Loader2 className="mr-1 h-3 w-3 animate-spin" />}
              {livePhase}
            </Badge>
            <Badge variant="outline" className="text-xs">
              StatefulSet
            </Badge>
          </>
        }
      />
      <Separator className="my-5" />

      {!isReady && (
        <DbWaitingPanel
          livePhase={livePhase}
          isStarting={isStarting}
          pods={pods}
          onDelete={() => setShowDelete(true)}
        />
      )}

      {isReady && (
        <Tabs defaultValue="overview">
          <TabsList>
            <TabsTrigger value="overview">Overview</TabsTrigger>
            <TabsTrigger value="backups" className="gap-1.5">
              Backups
              {backups.length > 0 && (
                <Badge variant="outline" className="ml-0.5 h-5 px-1.5 text-xs">
                  {backups.length}
                </Badge>
              )}
            </TabsTrigger>
            <TabsTrigger value="settings">Settings</TabsTrigger>
          </TabsList>

          <TabsContent value="overview" className="mt-4 space-y-6">
            <DbConnectionSection db={db} credentials={credentials} credsLoading={credsLoading} />
            <DbPodsSection pods={pods} />
          </TabsContent>

          <TabsContent value="backups" className="mt-4">
            <DbBackupsSection
              db={db}
              backups={backups}
              isReady={isReady}
              onTriggerBackup={() => triggerBackup.mutate()}
              isTriggering={triggerBackup.isPending}
              onRestore={(id) => restoreBackup.mutate(id)}
              isRestoring={restoreBackup.isPending}
            />
          </TabsContent>

          <TabsContent value="settings" className="mt-4">
            <DbSettingsSection db={db} onDelete={() => setShowDelete(true)} />
          </TabsContent>
        </Tabs>
      )}

      <DbDeleteDialog
        db={db}
        projectId={projectId}
        open={showDelete}
        onOpenChange={setShowDelete}
        confirmName={confirmName}
        onConfirmNameChange={setConfirmName}
        deleteDb={deleteDb}
      />
    </div>
  );
}
