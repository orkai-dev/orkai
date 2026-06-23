import { ChevronDown, Hash, Mail, MessageSquare, MessagesSquare, Send } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { LoadingScreen } from "@/components/loading-screen";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { ToggleSwitch } from "@/components/ui/toggle-switch";
import {
  useNotificationChannels,
  useNotifyEvents,
  useSaveChannel,
  useTestChannel,
} from "@/features/notifications";
import type { NotificationChannel, NotifyEventInfo } from "@/features/notifications/types";
import { cn } from "@/lib/utils";

// ── Notifications Tab ───────────────────────────────────────────────

const CHANNEL_DEFS = [
  {
    type: "email" as const,
    label: "Email",
    icon: Mail,
    note: "Requires SMTP configured in the SMTP tab",
    fields: [
      {
        key: "recipients",
        label: "Recipients",
        placeholder: "user@example.com, ops@example.com",
        multiline: true,
      },
    ],
  },
  {
    type: "telegram" as const,
    label: "Telegram",
    icon: Send,
    fields: [
      { key: "bot_token", label: "Bot Token", placeholder: "123456:ABC-DEF..." },
      { key: "chat_id", label: "Chat ID", placeholder: "-1001234567890" },
    ],
  },
  {
    type: "discord" as const,
    label: "Discord",
    icon: Hash,
    fields: [
      {
        key: "webhook_url",
        label: "Webhook URL",
        placeholder: "https://discord.com/api/webhooks/...",
      },
    ],
  },
  {
    type: "slack" as const,
    label: "Slack",
    icon: MessageSquare,
    fields: [
      {
        key: "webhook_url",
        label: "Webhook URL",
        placeholder: "https://hooks.slack.com/services/...",
      },
    ],
  },
  {
    type: "google_chat" as const,
    label: "Google Chat",
    icon: MessagesSquare,
    fields: [
      {
        key: "webhook_url",
        label: "Webhook URL",
        placeholder: "https://chat.googleapis.com/v1/spaces/.../messages?key=...&token=...",
      },
    ],
  },
];

function parseEventsFromConfig(config: Record<string, unknown> | undefined): Set<string> | null {
  const raw = config?.events;
  if (!Array.isArray(raw)) {
    return null;
  }
  return new Set(raw.filter((e): e is string => typeof e === "string"));
}

function fieldConfigFromExisting(
  def: (typeof CHANNEL_DEFS)[number],
  config: Record<string, unknown> | undefined,
): Record<string, string> {
  const out: Record<string, string> = {};
  for (const field of def.fields) {
    const v = config?.[field.key];
    out[field.key] = typeof v === "string" ? v : "";
  }
  return out;
}

function buildSaveConfig(
  fieldValues: Record<string, string>,
  selectedEvents: Set<string> | null,
): Record<string, unknown> {
  const config: Record<string, unknown> = { ...fieldValues };
  if (selectedEvents !== null) {
    config.events = [...selectedEvents];
  }
  return config;
}

function groupEventsByCategory(events: NotifyEventInfo[]): Map<string, NotifyEventInfo[]> {
  const map = new Map<string, NotifyEventInfo[]>();
  for (const e of events) {
    const list = map.get(e.category) ?? [];
    list.push(e);
    map.set(e.category, list);
  }
  return map;
}

export function NotificationsTab() {
  const { data: channels, isLoading: channelsLoading } = useNotificationChannels();
  const { data: events, isLoading: eventsLoading } = useNotifyEvents();

  if (channelsLoading || eventsLoading) return <LoadingScreen />;

  return (
    <Card>
      <CardContent className="divide-y p-0">
        {CHANNEL_DEFS.map((def) => {
          const existing = channels?.find((c) => c.type === def.type);
          return (
            <ChannelRow key={def.type} def={def} existing={existing} eventCatalog={events ?? []} />
          );
        })}
      </CardContent>
    </Card>
  );
}

function EventSelector({
  catalog,
  selectedEvents,
  onChange,
}: {
  catalog: NotifyEventInfo[];
  selectedEvents: Set<string> | null;
  onChange: (next: Set<string> | null) => void;
}) {
  const grouped = useMemo(() => groupEventsByCategory(catalog), [catalog]);
  const allEvents = selectedEvents === null;

  const toggleEvent = (key: string, on: boolean) => {
    const base = selectedEvents ?? new Set(catalog.map((e) => e.key));
    const next = new Set(base);
    if (on) {
      next.add(key);
    } else {
      next.delete(key);
    }
    onChange(next);
  };

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between gap-3">
        <div>
          <p className="text-sm font-medium">Events</p>
          <p className="text-xs text-muted-foreground">Choose which events this channel receives</p>
        </div>
        <div className="flex shrink-0 items-center gap-2">
          <span className="text-xs text-muted-foreground">All events</span>
          <ToggleSwitch
            checked={allEvents}
            onChange={(on) => {
              if (on) {
                onChange(null);
              } else {
                onChange(new Set(selectedEvents ?? catalog.map((e) => e.key)));
              }
            }}
          />
        </div>
      </div>
      {!allEvents && (
        <div className="space-y-4 rounded-md border bg-background p-3">
          {[...grouped.entries()].map(([category, items]) => (
            <div key={category} className="space-y-2">
              <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                {category}
              </p>
              <div className="space-y-2">
                {items.map((item) => (
                  <div key={item.key} className="flex items-center justify-between gap-3">
                    <span className="text-sm">{item.label}</span>
                    <ToggleSwitch
                      checked={selectedEvents?.has(item.key) ?? false}
                      onChange={(on) => toggleEvent(item.key, on)}
                    />
                  </div>
                ))}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

function eventFilterLabel(selectedEvents: Set<string> | null): string {
  if (selectedEvents === null) return "All events";
  if (selectedEvents.size === 0) return "No events";
  return `${selectedEvents.size} events`;
}

function ChannelRow({
  def,
  existing,
  eventCatalog,
}: {
  def: (typeof CHANNEL_DEFS)[number];
  existing?: NotificationChannel;
  eventCatalog: NotifyEventInfo[];
}) {
  const saveChannel = useSaveChannel();
  const testChannel = useTestChannel();
  const [enabled, setEnabled] = useState(existing?.enabled ?? false);
  const [expanded, setExpanded] = useState(false);
  const [config, setConfig] = useState<Record<string, string>>(() =>
    fieldConfigFromExisting(def, existing?.config),
  );
  const [selectedEvents, setSelectedEvents] = useState<Set<string> | null>(() =>
    parseEventsFromConfig(existing?.config),
  );

  useEffect(() => {
    if (existing) {
      setEnabled(existing.enabled);
      setConfig(fieldConfigFromExisting(def, existing.config));
      setSelectedEvents(parseEventsFromConfig(existing.config));
    }
  }, [existing, def]);

  const Icon = def.icon;

  const savePayload = () => buildSaveConfig(config, selectedEvents);

  const persist = (nextEnabled: boolean) => {
    saveChannel.mutate({
      type: def.type,
      enabled: nextEnabled,
      config: savePayload(),
    });
  };

  return (
    <div>
      <div className="flex flex-row items-center justify-between gap-3 px-4 py-3">
        <div className="flex min-w-0 flex-col gap-0.5">
          <div className="flex min-w-0 items-center gap-2">
            <Icon className="h-4 w-4 shrink-0 text-muted-foreground" />
            <span className="truncate text-sm font-medium">{def.label}</span>
          </div>
          <button
            type="button"
            className="truncate text-left text-xs text-muted-foreground hover:text-foreground"
            onClick={() => setExpanded(true)}
          >
            Configure webhook & events ({eventFilterLabel(selectedEvents)})
          </button>
        </div>
        <div className="flex shrink-0 items-center gap-2">
          <Badge variant={enabled ? "success" : "secondary"}>{enabled ? "On" : "Off"}</Badge>
          <Button
            variant="ghost"
            size="sm"
            className="h-8 gap-1 px-2 text-xs text-muted-foreground"
            onClick={() => setExpanded((v) => !v)}
          >
            Configure
            <ChevronDown
              className={cn("h-3.5 w-3.5 transition-transform", expanded && "rotate-180")}
            />
          </Button>
          <ToggleSwitch
            checked={enabled}
            onChange={(v) => {
              setEnabled(v);
              if (v) {
                setExpanded(true);
              }
              persist(v);
            }}
          />
        </div>
      </div>
      {expanded && (
        <div className="space-y-4 border-t bg-muted/20 px-4 py-4">
          {"note" in def && def.note && <p className="text-xs text-muted-foreground">{def.note}</p>}
          {def.fields.map((field) => (
            <div key={field.key} className="space-y-2">
              <Label>{field.label}</Label>
              {"multiline" in field && field.multiline ? (
                <textarea
                  className="flex min-h-[60px] w-full rounded-md border border-input bg-transparent px-3 py-2 text-sm shadow-sm placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
                  value={config[field.key] ?? ""}
                  onChange={(e) => setConfig((prev) => ({ ...prev, [field.key]: e.target.value }))}
                  placeholder={field.placeholder}
                />
              ) : (
                <Input
                  value={config[field.key] ?? ""}
                  onChange={(e) => setConfig((prev) => ({ ...prev, [field.key]: e.target.value }))}
                  placeholder={field.placeholder}
                />
              )}
            </div>
          ))}
          <EventSelector
            catalog={eventCatalog}
            selectedEvents={selectedEvents}
            onChange={setSelectedEvents}
          />
          <div className="flex gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={() => testChannel.mutate(def.type)}
              disabled={testChannel.isPending}
            >
              {testChannel.isPending ? "Testing..." : "Test"}
            </Button>
            <Button
              size="sm"
              onClick={() =>
                saveChannel.mutate({
                  type: def.type,
                  enabled,
                  config: savePayload(),
                })
              }
              disabled={saveChannel.isPending}
            >
              {saveChannel.isPending ? "Saving..." : "Save"}
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}
