import { ReactNode } from 'react';

/* Stat card — KPI/metric display with icon, value, label, and optional trend */

interface StatCardProps {
  label: string;
  value: string | number;
  icon?: ReactNode;
  color?: string;
  trend?: number;
  format?: (v: number) => string;
}

export default function StatCard({ label, value, icon, color = 'var(--brand)', trend }: StatCardProps) {
  return (
    <div className="card p-4 sm:p-5">
      <div className="flex items-center justify-between mb-3">
        {icon && (
          <div className="w-9 h-9 rounded-lg flex items-center justify-center"
            style={{ background: `color-mix(in srgb, ${color} 10%, transparent)`, color }}>
            {icon}
          </div>
        )}
        {trend !== undefined && trend !== 0 && (
          <span className="flex items-center gap-0.5 text-xs font-semibold px-1.5 py-0.5 rounded-full"
            style={{
              color: trend > 0 ? 'var(--success)' : 'var(--error)',
              background: trend > 0 ? 'var(--success-muted)' : 'var(--error-muted)',
            }}>
            <svg width="10" height="10" viewBox="0 0 10 10" fill="none">
              <path d={trend > 0 ? "M5 2L8 6H2L5 2Z" : "M5 8L2 4H8L5 8Z"} fill="currentColor" />
            </svg>
            {Math.abs(trend)}
          </span>
        )}
      </div>
      <div className="text-2xl sm:text-3xl font-display font-bold tabular-nums" style={{ color: 'var(--text-primary)' }}>
        {value}
      </div>
      <div className="text-xs font-medium mt-1" style={{ color: 'var(--text-tertiary)' }}>
        {label}
      </div>
    </div>
  );
}
