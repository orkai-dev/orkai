import { DangerZone } from "@/components/danger-zone";
import type { ManagedDB } from "@/features/databases/types";
import { DbExternalAccessSection } from "./-db-external-access-section";

export function DbSettingsSection({ db, onDelete }: { db: ManagedDB; onDelete: () => void }) {
  return (
    <div className="space-y-6">
      <DbExternalAccessSection db={db} />

      <DangerZone
        description="Delete this database. All data will be permanently lost."
        buttonLabel="Delete Database"
        onDelete={onDelete}
      />
    </div>
  );
}
