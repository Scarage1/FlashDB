/* FlashDB Logo — Lightning bolt emerging from database layers
   Conveys: Speed (bolt), Data (layers), Modern (geometric, clean) */
export function FlashLogo({ size = 32, className = '' }: { size?: number; className?: string }) {
  return (
    <svg width={size} height={size} viewBox="0 0 40 40" fill="none" className={className} xmlns="http://www.w3.org/2000/svg">
      <defs>
        <linearGradient id="flash-grad" x1="0%" y1="0%" x2="100%" y2="100%">
          <stop offset="0%" stopColor="#f59e0b" />
          <stop offset="100%" stopColor="#ef4444" />
        </linearGradient>
        <linearGradient id="flash-layer" x1="0%" y1="0%" x2="100%" y2="100%">
          <stop offset="0%" stopColor="#f59e0b" stopOpacity="0.3" />
          <stop offset="100%" stopColor="#ef4444" stopOpacity="0.1" />
        </linearGradient>
      </defs>
      {/* Base rounded square */}
      <rect x="2" y="2" width="36" height="36" rx="10" fill="url(#flash-grad)" />
      {/* Database layers */}
      <ellipse cx="20" cy="28" rx="10" ry="3" fill="white" fillOpacity="0.2" />
      <ellipse cx="20" cy="24" rx="10" ry="3" fill="white" fillOpacity="0.25" />
      {/* Lightning bolt */}
      <path d="M22 8L14 22h6l-2 10 8-14h-6l2-10z" fill="white" />
    </svg>
  );
}

export function FlashLogoMark({ size = 20, className = '' }: { size?: number; className?: string }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none" className={className} xmlns="http://www.w3.org/2000/svg">
      <path d="M13 2L5 14h5l-1 8 8-12h-5l1-8z" fill="url(#flash-mark-grad)" />
      <defs>
        <linearGradient id="flash-mark-grad" x1="0%" y1="0%" x2="100%" y2="100%">
          <stop offset="0%" stopColor="#f59e0b" />
          <stop offset="100%" stopColor="#ef4444" />
        </linearGradient>
      </defs>
    </svg>
  );
}
