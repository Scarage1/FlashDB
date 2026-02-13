/* StatusDot — live/dead indicator with optional pulse animation */

interface StatusDotProps {
  online: boolean;
  size?: number;
  pulse?: boolean;
  label?: string;
}

export default function StatusDot({ online, size = 8, pulse = true, label }: StatusDotProps) {
  const color = online ? 'var(--success)' : 'var(--error)';
  return (
    <span className="inline-flex items-center gap-1.5">
      <span className="relative flex" style={{ width: size, height: size }}>
        {online && pulse && (
          <span className="absolute inline-flex h-full w-full rounded-full opacity-75"
            style={{ background: color, animation: 'pulse-dot 1.5s cubic-bezier(0,0,0.2,1) infinite' }} />
        )}
        <span className="relative inline-flex rounded-full" style={{ width: size, height: size, background: color }} />
      </span>
      {label && (
        <span className="text-xs font-medium" style={{ color }}>
          {label}
        </span>
      )}
    </span>
  );
}
