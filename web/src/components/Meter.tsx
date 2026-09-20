export function Meter({
  value,
  tone = 'mint',
}: {
  value: number | null | undefined;
  tone?: string;
}) {
  return (
    <div className={'meter ' + tone}>
      <span style={{ width: `${Math.max(0, Math.min(100, value || 0))}%` }} />
    </div>
  );
}
