const LABELS: Record<string, string> = {
  running: "运行中",
  stopped: "已停用",
  error: "异常",
  draft: "草稿",
  enabled: "已启用",
};

export function StatusBadge({ status }: { status: string }) {
  const label = LABELS[status] || status;
  return (
    <span className={`status ${status}`} title={label}>
      <span aria-hidden="true" />
      {label}
    </span>
  );
}
