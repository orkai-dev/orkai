import type { FeatureModule } from "@/app/registry";

export const monitoringFeature: FeatureModule = {
  id: "monitoring",
  invalidations: {
    alerts: [["monitoring"]],
  },
};
