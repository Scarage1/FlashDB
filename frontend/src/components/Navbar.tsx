'use client';

import { useState, useEffect } from 'react';
import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { useTheme } from '@/context/ThemeContext';
import { FlashLogo } from '@/components/ui/Logo';
import StatusDot from '@/components/ui/StatusDot';
import { Sun, Moon, Search, Menu, X, Flame, LineChart, Camera, Radio, Gauge, Settings, ChevronDown, ExternalLink } from 'lucide-react';

interface NavbarProps { onCommandPalette: () => void; }

const mainNav = [
  { label: 'Dashboard', href: '/' },
  { label: 'Console', href: '/console' },
  { label: 'Explorer', href: '/explorer' },
  { label: 'Monitoring', href: '/monitoring' },
];

const toolsNav = [
  { label: 'Hot Keys', href: '/hotkeys', icon: <Flame size={15} />, desc: 'Access frequency analysis', color: '#ef4444' },
  { label: 'Time Series', href: '/timeseries', icon: <LineChart size={15} />, desc: 'Temporal data management', color: '#0284c7' },
  { label: 'Snapshots', href: '/snapshots', icon: <Camera size={15} />, desc: 'Point-in-time backups', color: '#8b5cf6' },
  { label: 'CDC Stream', href: '/cdc', icon: <Radio size={15} />, desc: 'Change data capture', color: '#f59e0b' },
  { label: 'Benchmark', href: '/benchmark', icon: <Gauge size={15} />, desc: 'Performance testing', color: '#10b981' },
];

export default function Navbar({ onCommandPalette }: NavbarProps) {
  const pathname = usePathname();
  const { theme, toggleTheme } = useTheme();
  const [scrolled, setScrolled] = useState(false);
  const [serverOnline, setServerOnline] = useState<boolean | null>(null);
  const [mobileOpen, setMobileOpen] = useState(false);
  const [toolsOpen, setToolsOpen] = useState(false);

  useEffect(() => {
    const onScroll = () => setScrolled(window.scrollY > 8);
    window.addEventListener('scroll', onScroll, { passive: true });
    return () => window.removeEventListener('scroll', onScroll);
  }, []);

  useEffect(() => {
    const check = async () => {
      try { const r = await fetch('/api/v1/stats'); setServerOnline(r.ok); }
      catch { setServerOnline(false); }
    };
    check(); const id = setInterval(check, 5000);
    return () => clearInterval(id);
  }, []);

  useEffect(() => { setMobileOpen(false); setToolsOpen(false); }, [pathname]);

  const isActive = (href: string) => href === '/' ? pathname === '/' : pathname.startsWith(href);

  return (
    <header className={`sticky top-0 z-50 glass transition-all duration-200 ${scrolled ? 'shadow-sm' : ''}`}
      style={{ borderBottom: `1px solid ${scrolled ? 'var(--border)' : 'var(--nav-border)'}` }}>
      <div className="max-w-7xl mx-auto px-4 sm:px-6">
        <div className="flex items-center justify-between h-16">
          {/* Logo */}
          <Link href="/" className="flex items-center gap-2.5 group flex-shrink-0">
            <FlashLogo size={32} />
            <span className="font-display text-lg font-bold tracking-tight" style={{ color: 'var(--text-primary)' }}>
              Flash<span className="text-gradient">DB</span>
            </span>
          </Link>

          {/* Desktop Nav */}
          <nav className="hidden lg:flex items-center gap-0.5 ml-10">
            {mainNav.map((link) => (
              <Link key={link.href} href={link.href}
                className={`relative px-3 py-1.5 rounded-lg text-sm font-medium transition-colors ${isActive(link.href) ? '' : 'hover:bg-[var(--bg-secondary)]'}`}
                style={{ color: isActive(link.href) ? 'var(--brand)' : 'var(--text-secondary)' }}>
                {link.label}
                {isActive(link.href) && (
                  <span className="absolute inset-x-2 -bottom-[17px] h-0.5 rounded-full bg-gradient-brand" />
                )}
              </Link>
            ))}

            {/* Tools dropdown */}
            <div className="relative"
              onMouseEnter={() => setToolsOpen(true)}
              onMouseLeave={() => setToolsOpen(false)}>
              <button
                className={`flex items-center gap-1 px-3 py-1.5 rounded-lg text-sm font-medium transition-colors ${toolsOpen ? 'bg-[var(--bg-secondary)]' : 'hover:bg-[var(--bg-secondary)]'}`}
                style={{ color: toolsNav.some(l => isActive(l.href)) ? 'var(--brand)' : 'var(--text-secondary)' }}>
                Tools
                <ChevronDown size={13} className={`transition-transform duration-200 ${toolsOpen ? 'rotate-180' : ''}`} />
                {toolsNav.some(l => isActive(l.href)) && (
                  <span className="absolute inset-x-2 -bottom-[17px] h-0.5 rounded-full bg-gradient-brand" />
                )}
              </button>

              {toolsOpen && (
                <div className="absolute top-full left-0 pt-2" style={{ width: '280px' }}>
                  <div className="rounded-xl p-2 shadow-xl animate-scale-in"
                    style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border)', boxShadow: 'var(--shadow-xl)' }}>
                    {toolsNav.map((link) => (
                      <Link key={link.href} href={link.href}
                        className="flex items-start gap-3 px-3 py-2.5 rounded-lg transition-colors hover:bg-[var(--bg-secondary)]"
                        style={{ color: isActive(link.href) ? 'var(--brand)' : 'var(--text-primary)' }}>
                        <span className="w-8 h-8 rounded-lg flex items-center justify-center flex-shrink-0 mt-0.5"
                          style={{ background: `color-mix(in srgb, ${link.color} 10%, transparent)`, color: link.color }}>
                          {link.icon}
                        </span>
                        <div className="min-w-0">
                          <div className="text-sm font-semibold">{link.label}</div>
                          <div className="text-xs mt-0.5" style={{ color: 'var(--text-tertiary)' }}>{link.desc}</div>
                        </div>
                      </Link>
                    ))}
                    <div className="my-1.5 mx-2" style={{ borderTop: '1px solid var(--border)' }} />
                    <Link href="/settings"
                      className="flex items-center gap-2 px-3 py-2 rounded-lg text-sm transition-colors hover:bg-[var(--bg-secondary)]"
                      style={{ color: isActive('/settings') ? 'var(--brand)' : 'var(--text-secondary)' }}>
                      <Settings size={14} /> Settings
                    </Link>
                  </div>
                </div>
              )}
            </div>
          </nav>

          {/* Right side */}
          <div className="flex items-center gap-2">
            {/* Search */}
            <button onClick={onCommandPalette}
              className="hidden sm:flex items-center gap-2 px-3 py-1.5 rounded-lg text-xs transition-colors hover:bg-[var(--bg-secondary)]"
              style={{ color: 'var(--text-tertiary)', border: '1px solid var(--border)' }}>
              <Search size={13} />
              <span>Search</span>
              <kbd className="px-1.5 py-0.5 rounded text-[10px] font-mono ml-1"
                style={{ background: 'var(--bg-tertiary)', color: 'var(--text-tertiary)' }}>⌘K</kbd>
            </button>

            {/* Status */}
            {serverOnline !== null && (
              <div className="hidden sm:flex items-center px-2.5 py-1.5 rounded-lg" style={{ border: '1px solid var(--border)' }}>
                <StatusDot online={serverOnline} label={serverOnline ? 'Online' : 'Offline'} />
              </div>
            )}

            {/* Theme */}
            <button onClick={toggleTheme}
              className="w-8 h-8 rounded-lg flex items-center justify-center transition-colors hover:bg-[var(--bg-secondary)]"
              style={{ color: 'var(--text-secondary)', border: '1px solid var(--border)' }}>
              {theme === 'dark' ? <Sun size={15} /> : <Moon size={15} />}
            </button>

            {/* GitHub */}
            <a href="https://github.com/Scarage1/FlashDB" target="_blank" rel="noopener noreferrer"
              className="hidden md:flex w-8 h-8 rounded-lg items-center justify-center transition-colors hover:bg-[var(--bg-secondary)]"
              style={{ color: 'var(--text-secondary)', border: '1px solid var(--border)' }}>
              <ExternalLink size={14} />
            </a>

            {/* Mobile menu */}
            <button onClick={() => setMobileOpen(!mobileOpen)}
              className="lg:hidden w-8 h-8 rounded-lg flex items-center justify-center"
              style={{ color: 'var(--text-secondary)', border: '1px solid var(--border)' }}>
              {mobileOpen ? <X size={16} /> : <Menu size={16} />}
            </button>
          </div>
        </div>
      </div>

      {/* Mobile menu */}
      {mobileOpen && (
        <div className="lg:hidden px-4 pb-4 pt-2 animate-slide-up"
          style={{ borderTop: '1px solid var(--border)', background: 'var(--bg-primary)' }}>
          {[...mainNav, ...toolsNav.map(t => ({ label: t.label, href: t.href })), { label: 'Settings', href: '/settings' }].map((link) => (
            <Link key={link.href} href={link.href}
              className="block px-3 py-2.5 rounded-lg text-sm font-medium transition-colors"
              style={{ color: isActive(link.href) ? 'var(--brand)' : 'var(--text-primary)', background: isActive(link.href) ? 'var(--brand-muted)' : 'transparent' }}>
              {link.label}
            </Link>
          ))}
        </div>
      )}
    </header>
  );
}
