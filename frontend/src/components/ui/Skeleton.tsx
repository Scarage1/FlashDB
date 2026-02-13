/* Skeleton loading components — show structure before data loads */

export function SkeletonLine({ width = '100%', height = '16px' }: { width?: string; height?: string }) {
  return <div className="skeleton" style={{ width, height }} />;
}

export function SkeletonCard() {
  return (
    <div className="card p-5 space-y-3">
      <SkeletonLine width="40%" height="12px" />
      <SkeletonLine width="60%" height="24px" />
      <SkeletonLine width="80%" height="12px" />
    </div>
  );
}

export function SkeletonTable({ rows = 5 }: { rows?: number }) {
  return (
    <div className="card overflow-hidden">
      <div className="px-5 py-3" style={{ borderBottom: '1px solid var(--border)' }}>
        <SkeletonLine width="120px" height="14px" />
      </div>
      {Array.from({ length: rows }).map((_, i) => (
        <div key={i} className="flex items-center gap-4 px-5 py-3" style={{ borderTop: i > 0 ? '1px solid var(--border)' : undefined }}>
          <SkeletonLine width="60px" height="12px" />
          <SkeletonLine width="40%" height="12px" />
          <div className="flex-1" />
          <SkeletonLine width="80px" height="12px" />
        </div>
      ))}
    </div>
  );
}

export function SkeletonChart({ height = 120 }: { height?: number }) {
  return (
    <div className="card p-5">
      <div className="flex items-center gap-2 mb-4">
        <SkeletonLine width="100px" height="14px" />
        <div className="flex-1" />
        <SkeletonLine width="60px" height="12px" />
      </div>
      <div className="skeleton" style={{ width: '100%', height: `${height}px` }} />
    </div>
  );
}
