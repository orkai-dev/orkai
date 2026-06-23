import { ConfirmDialog } from "@/components/confirm-dialog";
import { useDeleteApp } from "@/features/apps";
import { useDeleteCronJob } from "@/features/cronjobs";
import { useDeleteDatabase } from "@/features/databases";
import { useDeletePage } from "@/features/pages/queries";
import { useDeleteWorker } from "@/features/workers/queries";
import type { ServiceItem } from "./-service-types";

export function DeleteServiceDialog({
  service,
  onClose,
}: {
  service: ServiceItem;
  onClose: () => void;
}) {
  const deleteApp = useDeleteApp(service.data.id);
  const deleteDb = useDeleteDatabase(service.data.id);
  const deletePage = useDeletePage(service.data.id);
  const deleteWorker = useDeleteWorker(service.data.id);
  const deleteCronJob = useDeleteCronJob(service.data.id);

  const { mutation, removes } = (() => {
    switch (service.type) {
      case "app":
        return { mutation: deleteApp, removes: "Deployment" };
      case "page":
        return { mutation: deletePage, removes: "S3 bucket and CloudFront distribution" };
      case "worker":
        return { mutation: deleteWorker, removes: "Cloudflare Worker script" };
      case "database":
        return { mutation: deleteDb, removes: "StatefulSet" };
      case "cronjob":
        return { mutation: deleteCronJob, removes: "CronJob" };
    }
  })();

  return (
    <ConfirmDialog
      open
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
      title="Delete Service"
      description={
        <>
          Delete <strong>{service.data.name}</strong>? This removes the {removes}.
        </>
      }
      confirmLabel="Delete"
      loading={mutation.isPending}
      onConfirm={() => mutation.mutate(undefined as any, { onSuccess: onClose })}
    />
  );
}
