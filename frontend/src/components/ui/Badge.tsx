/* Badge — small label component for status, types, categories */

interface BadgeProps {
  children: React.ReactNode;
  color?: string;
  variant?: 'solid' | 'soft' | 'outline';
}

export default function Badge({ children, color = 'var(--brand)', variant = 'soft' }: BadgeProps) {
  const styles = {
    solid: { background: color, color: 'white', border: 'none' },
    soft: { background: `color-mix(in srgb, ${color} 10%, transparent)`, color, border: 'none' },
    outline: { background: 'transparent', color, border: `1px solid ${color}` },
  };

  return (
    <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-md text-xs font-semibold"
      style={styles[variant]}>
      {children}
    </span>
  );
}
