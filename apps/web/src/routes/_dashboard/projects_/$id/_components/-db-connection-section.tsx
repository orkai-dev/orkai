import { Shield } from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import type { DatabaseCredentials, ManagedDB } from "@/features/databases/types";
import { DbCredentialRow } from "./-db-credential-row";

export function DbConnectionSection({
  db,
  credentials,
  credsLoading,
}: {
  db: ManagedDB;
  credentials: DatabaseCredentials | undefined;
  credsLoading: boolean;
}) {
  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-sm font-medium">
          <Shield className="h-4 w-4" /> Connection Info
        </CardTitle>
      </CardHeader>
      <CardContent>
        {credsLoading ? (
          <div className="space-y-3">
            {Array.from({ length: 6 }).map((_, i) => (
              <Skeleton key={i} className="h-5 w-full" />
            ))}
          </div>
        ) : credentials ? (
          <div className="divide-y">
            <DbCredentialRow label="Host" value={credentials.host} mono />
            <DbCredentialRow label="Port" value={String(credentials.port)} mono />
            <DbCredentialRow label="Username" value={credentials.username} mono />
            <DbCredentialRow label="Password" value={credentials.password} secret mono />
            <DbCredentialRow label="Database" value={credentials.database_name} mono />
            <DbCredentialRow
              label="Connection String"
              value={credentials.connection_string}
              secret
              mono
            />
            <DbCredentialRow label="Internal URL" value={credentials.internal_url} mono />
            {db.external_enabled && db.external_port > 0 && (
              <DbCredentialRow
                label="External URL"
                value={`${window.location.hostname}:${db.external_port}`}
                mono
              />
            )}
          </div>
        ) : (
          <p className="text-sm text-muted-foreground">
            Credentials will be available once the database is deployed and running.
          </p>
        )}
      </CardContent>
    </Card>
  );
}
