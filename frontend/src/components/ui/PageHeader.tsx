import { ReactNode } from 'react';

/* Page header — consistent header across all pages */

interface PageHeaderProps {
  title: string;
  description: string;
  badge?: ReactNode;
  actions?: ReactNode;
}

export default function PageHeader({ title, description, badge, actions }: PageHeaderProps) {
  return (
    <div className="flex items-start sm:items-center justify-between mb-8 gap-4 flex-col sm:flex-row">
      <div>
        <div className="flex items-center gap-3">
          <h1 className="font-display text-2xl sm:text-3xl font-bold tracking-tight" style={{ color: 'var(--text-primary)' }}>
            {title}
          </h1>
          {badge && (
            typeof badge === 'string'
              ? <span className="text-[11px] font-semibold px-2 py-0.5 rounded-full"
                  style={{ background: 'var(--brand-muted)', color: 'var(--brand)' }}>{badge}</span>
              : badge
          )}
        </div>
        <p className="text-sm mt-1.5" style={{ color: 'var(--text-secondary)' }}>
          {description}
        </p>
      </div>
      {actions && <div className="flex items-center gap-2 flex-shrink-0">{actions}</div>}
    </div>
  );
}
