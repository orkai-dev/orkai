import { Link } from "@tanstack/react-router";
import { useCallback, useEffect, useState } from "react";
import { BucketCombobox } from "@/components/ui/bucket-combobox";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { Textarea } from "@/components/ui/textarea";
import {
  useCreateResource,
  useResourceBuckets,
  useResources,
  useUpdateResource,
} from "@/features/resources";
import type { ResourceTag, SharedResource } from "@/features/resources/types";
import {
  CLOUD_ACCOUNT_FIELDS_BY_PROVIDER,
  FIELDS,
  PROVIDER_OPTIONS,
  REGISTRY_FIELDS_BY_PROVIDER,
  type ResourceType,
} from "./-resources.config";
import {
  buildResourceConfig,
  configWithoutTags,
  extractTagsFromConfig,
  TagsEditor,
} from "./-tags-editor";

export function ResourceSheet({
  open,
  onOpenChange,
  type,
  resource,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  type: ResourceType;
  resource: SharedResource | null;
}) {
  const isEdit = !!resource;
  const createMutation = useCreateResource();
  const updateMutation = useUpdateResource();

  const [name, setName] = useState("");
  const [provider, setProvider] = useState("");
  const [config, setConfig] = useState<Record<string, string>>({});
  const [tags, setTags] = useState<ResourceTag[]>([]);
  const [tagsEditorKey, setTagsEditorKey] = useState(0);

  // Sync form state when resource changes (edit mode) or sheet opens
  useEffect(() => {
    if (open) {
      setTagsEditorKey((k) => k + 1);
      if (resource) {
        setName(resource.name);
        setProvider(resource.provider);
        const raw = typeof resource.config === "object" && resource.config ? resource.config : {};
        const initial = configWithoutTags(raw);
        // A cloud-account-backed ECR registry persists only {cloud_account_id,
        // region} (no auth_mode), so derive the toggle from the reference.
        if (
          resource.type === "registry" &&
          resource.provider === "ecr" &&
          initial.cloud_account_id &&
          !initial.auth_mode
        ) {
          initial.auth_mode = "cloud_account";
        }
        setConfig(initial);
        setTags(extractTagsFromConfig(raw));
      } else {
        setName("");
        setProvider(PROVIDER_OPTIONS[type]?.[0]?.value ?? "");
        setConfig({});
        setTags([]);
      }
    }
  }, [open, resource, type]);

  const setConfigField = useCallback((key: string, value: string) => {
    setConfig((prev) => ({ ...prev, [key]: value }));
  }, []);

  const saving = createMutation.isPending || updateMutation.isPending;

  const handleSubmit = useCallback(
    (e: React.FormEvent) => {
      e.preventDefault();
      // For ECR, drop the fields that don't belong to the selected auth mode so
      // toggling never sends both static keys and a cloud-account reference.
      let effectiveConfig = config;
      if (type === "registry" && provider === "ecr") {
        const { access_key, secret_key, cloud_account_id, ...rest } = config;
        effectiveConfig =
          config.auth_mode === "cloud_account"
            ? { ...rest, cloud_account_id }
            : { ...rest, access_key, secret_key };
      }
      const payload = buildResourceConfig(effectiveConfig, tags);
      if (isEdit && resource) {
        updateMutation.mutate(
          { id: resource.id, name, provider, config: payload },
          { onSuccess: () => onOpenChange(false) },
        );
      } else {
        createMutation.mutate(
          { name, type, provider, config: payload },
          { onSuccess: () => onOpenChange(false) },
        );
      }
    },
    [
      isEdit,
      resource,
      name,
      provider,
      config,
      tags,
      type,
      createMutation,
      updateMutation,
      onOpenChange,
    ],
  );

  // S3 object storage can be backed by a connected AWS cloud account: the
  // operator picks an account and a bucket, and the API resolves credentials.
  // Only use this flow for new resources or ones already linked to an account —
  // existing resources with manually-entered keys keep the regular field inputs
  // so they remain fully editable.
  const isS3FromAccount =
    type === "object_storage" && provider === "aws_s3" && (!isEdit || !!config.cloud_account_id);
  // ECR can resolve credentials from a connected AWS cloud account instead of
  // static keys; the operator picks the account and a region.
  const isEcrFromAccount =
    type === "registry" && provider === "ecr" && config.auth_mode === "cloud_account";
  const { data: cloudAccounts } = useResources("cloud_account");
  const accounts = (cloudAccounts ?? []).filter(
    (a: SharedResource) => a.type === "cloud_account" && a.provider === "aws",
  );
  const cloudAccountId = config.cloud_account_id ?? "";
  const {
    data: buckets,
    isFetching: bucketsLoading,
    error: bucketsError,
  } = useResourceBuckets(isS3FromAccount ? cloudAccountId : "");

  // Block submit in cloud-account mode until an account is picked, otherwise the
  // registry would persist with no credentials and only fail later at pull/test
  // time with an opaque AWS credential-chain error.
  const submitDisabled = saving || (isEcrFromAccount && !cloudAccountId);

  const fields =
    type === "registry"
      ? (REGISTRY_FIELDS_BY_PROVIDER[provider] ?? FIELDS.registry)
      : type === "cloud_account"
        ? (CLOUD_ACCOUNT_FIELDS_BY_PROVIDER[provider] ?? FIELDS.cloud_account)
        : FIELDS[type];
  const providers = PROVIDER_OPTIONS[type];

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent>
        <SheetHeader>
          <SheetTitle>
            {isEdit ? "Edit" : "Add"} {type.replace("_", " ")}
          </SheetTitle>
          <SheetDescription>
            {isEdit ? "Update the resource configuration." : "Connect a new resource."}
          </SheetDescription>
        </SheetHeader>

        <form onSubmit={handleSubmit} className="flex flex-1 flex-col gap-4 overflow-y-auto">
          <div className="space-y-1">
            <Label htmlFor="res-name">Name</Label>
            <Input
              id="res-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="Auto-generated"
            />
            <p className="text-xs text-muted-foreground">Leave empty to auto-generate</p>
          </div>

          {providers.length > 1 && (
            <div className="space-y-1">
              <Label>Provider</Label>
              <Select value={provider} onValueChange={setProvider}>
                <SelectTrigger>
                  <SelectValue placeholder="Select provider" />
                </SelectTrigger>
                <SelectContent>
                  {providers.map((p) => (
                    <SelectItem key={p.value} value={p.value}>
                      {p.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          )}

          {isS3FromAccount ? (
            <>
              <div className="space-y-1">
                <Label>AWS account</Label>
                {accounts.length === 0 ? (
                  <p className="text-sm text-muted-foreground">
                    No AWS accounts configured.{" "}
                    <Link
                      to="/admin/resources"
                      search={{ tab: "cloud_account" }}
                      className="text-primary underline underline-offset-4 hover:text-primary/80"
                    >
                      Add one first
                    </Link>
                    .
                  </p>
                ) : (
                  <Select
                    value={cloudAccountId}
                    onValueChange={(v) => {
                      setConfig((prev) => ({ ...prev, cloud_account_id: v, bucket: "" }));
                    }}
                  >
                    <SelectTrigger>
                      <SelectValue placeholder="Select AWS account" />
                    </SelectTrigger>
                    <SelectContent>
                      {accounts.map((a: SharedResource) => (
                        <SelectItem key={a.id} value={a.id}>
                          {a.name}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                )}
                <p className="text-xs text-muted-foreground">
                  Credentials are resolved from the selected account — no keys to re-enter.
                </p>
              </div>

              {cloudAccountId && (
                <div className="space-y-1">
                  <Label>Bucket</Label>
                  <BucketCombobox
                    buckets={buckets ?? []}
                    value={config.bucket ?? ""}
                    onSelect={(b) => setConfigField("bucket", b)}
                    loading={bucketsLoading}
                    error={
                      bucketsError
                        ? (bucketsError as Error).message || "Failed to list buckets"
                        : null
                    }
                  />
                  <p className="text-xs text-muted-foreground">
                    Search your account's S3 buckets, or type a name to use one directly.
                  </p>
                </div>
              )}
            </>
          ) : (
            <>
              {fields
                .filter((f) => !f.showIf || f.showIf(config))
                .map((f) => (
                  <div key={f.key} className="space-y-1">
                    <Label htmlFor={f.type === "tags" ? undefined : `res-${f.key}`}>
                      {f.label}
                    </Label>
                    {f.type === "tags" ? (
                      <TagsEditor key={tagsEditorKey} tags={tags} onChange={setTags} />
                    ) : f.type === "textarea" ? (
                      <Textarea
                        id={`res-${f.key}`}
                        value={config[f.key] ?? ""}
                        onChange={(e) => setConfigField(f.key, e.target.value)}
                        placeholder={f.placeholder}
                        required={f.required}
                        rows={6}
                        className="font-mono text-xs"
                      />
                    ) : f.type === "select" ? (
                      <Select
                        value={config[f.key] ?? f.options?.[0]?.value ?? ""}
                        onValueChange={(v) => setConfigField(f.key, v)}
                      >
                        <SelectTrigger id={`res-${f.key}`}>
                          <SelectValue placeholder={f.placeholder} />
                        </SelectTrigger>
                        <SelectContent>
                          {f.options?.map((o) => (
                            <SelectItem key={o.value} value={o.value}>
                              {o.label}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    ) : (
                      <Input
                        id={`res-${f.key}`}
                        type={f.type}
                        value={config[f.key] ?? ""}
                        onChange={(e) => setConfigField(f.key, e.target.value)}
                        placeholder={f.placeholder}
                        required={f.required}
                      />
                    )}
                    {f.help && <p className="text-xs text-muted-foreground">{f.help}</p>}
                  </div>
                ))}

              {isEcrFromAccount && (
                <div className="space-y-1">
                  <Label>AWS account</Label>
                  {accounts.length === 0 ? (
                    <p className="text-sm text-muted-foreground">
                      No AWS accounts configured.{" "}
                      <Link
                        to="/admin/resources"
                        search={{ tab: "cloud_account" }}
                        className="text-primary underline underline-offset-4 hover:text-primary/80"
                      >
                        Add one first
                      </Link>
                      .
                    </p>
                  ) : (
                    <Select
                      value={config.cloud_account_id ?? ""}
                      onValueChange={(v) => setConfigField("cloud_account_id", v)}
                    >
                      <SelectTrigger>
                        <SelectValue placeholder="Select AWS account" />
                      </SelectTrigger>
                      <SelectContent>
                        {accounts.map((a: SharedResource) => (
                          <SelectItem key={a.id} value={a.id}>
                            {a.name}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  )}
                  <p className="text-xs text-muted-foreground">
                    Credentials are resolved from the selected account — no keys to re-enter.
                  </p>
                </div>
              )}
            </>
          )}

          <div className="mt-auto pt-4">
            <Button type="submit" className="w-full" disabled={submitDisabled}>
              {saving ? "Saving..." : isEdit ? "Update" : "Create"}
            </Button>
          </div>
        </form>
      </SheetContent>
    </Sheet>
  );
}
