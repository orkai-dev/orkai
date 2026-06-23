// Database engine display labels. Re-exported from `@/lib/constants` for
// backward compatibility during the feature-module migration.

export const ENGINE_LABELS: Record<string, string> = {
  postgres: "PostgreSQL",
  mysql: "MySQL",
  mariadb: "MariaDB",
  redis: "Redis",
  valkey: "Valkey",
  mongo: "MongoDB",
};
