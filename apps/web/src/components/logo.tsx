import { BRAND_NAME } from "@/lib/brand";

export function Logo({ className }: { className?: string }) {
  return <img src="/favicon.svg" alt={BRAND_NAME} className={className} draggable={false} />;
}
