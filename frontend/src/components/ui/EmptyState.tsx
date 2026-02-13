import { ReactNode } from 'react';

/* Empty state — shown when a section has no data.
   Provides illustration + guidance + optional CTA */

interface EmptyStateProps {
  icon: ReactNode;
  title: string;
  description: string;
  action?: ReactNode;
}

export default function EmptyState({ icon, title, description, action }: EmptyStateProps) {
  return (
    <div className="flex flex-col items-center justify-center py-16 px-6 text-center">
      <div className="w-16 h-16 rounded-2xl flex items-center justify-center mb-5"
        style={{ background: 'var(--bg-tertiary)', color: 'var(--text-tertiary)' }}>
        {icon}
      </div>
      <h3 className="font-display text-lg font-semibold mb-2" style={{ color: 'var(--text-primary)' }}>
        {title}
      </h3>
      <p className="text-sm max-w-sm leading-relaxed mb-6" style={{ color: 'var(--text-tertiary)' }}>
        {description}
      </p>
      {action}
    </div>
  );
}
