import { Badge } from "@/src/components/ui";

type ChipTone = "success" | "danger" | "warning" | "default";

function resolveTone(status: string): ChipTone {
  const normalized = (status || "unknown").toLowerCase();

  if (normalized.includes("active") || normalized.includes("running") || normalized.includes("enabled")) {
    return "success";
  }
  if (normalized.includes("failed") || normalized.includes("error") || normalized.includes("disabled")) {
    return "danger";
  }
  if (normalized.includes("inactive") || normalized.includes("stopped")) {
    return "warning";
  }
  return "default";
}

export function StatusChip({ status }: { status: string }) {
  const tone = resolveTone(status);
  return <Badge variant={tone}>{status || "unknown"}</Badge>;
}
