import type { BadgeProps } from "@/components/ui/badge";

export const AVATAR_EMOJI: Record<string, string> = {
  bear: "\u{1F43B}",
  cat: "\u{1F431}",
  dog: "\u{1F436}",
  fox: "\u{1F98A}",
  koala: "\u{1F428}",
  lion: "\u{1F981}",
  monkey: "\u{1F435}",
  owl: "\u{1F989}",
  panda: "\u{1F43C}",
  penguin: "\u{1F427}",
  rabbit: "\u{1F430}",
  tiger: "\u{1F42F}",
  whale: "\u{1F433}",
  wolf: "\u{1F43A}",
};

export function roleVariant(role: string): NonNullable<BadgeProps["variant"]> {
  switch (role.toLowerCase()) {
    case "admin":
      return "outline";
    default:
      return "secondary";
  }
}

export function relativeExpiry(expiresAt: string): string {
  const now = Date.now();
  const expires = new Date(expiresAt).getTime();
  const diffMs = expires - now;
  if (diffMs <= 0) return "Expired";
  const days = Math.floor(diffMs / (1000 * 60 * 60 * 24));
  const hours = Math.floor((diffMs % (1000 * 60 * 60 * 24)) / (1000 * 60 * 60));
  if (days > 0) return `Expires in ${days}d`;
  if (hours > 0) return `Expires in ${hours}h`;
  return "Expires soon";
}
