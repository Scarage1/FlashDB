'use client';

import { useState, useEffect, useRef, useCallback } from 'react';
import { useRouter } from 'next/navigation';
import { motion, AnimatePresence } from 'framer-motion';
import { executeCommand } from '@/lib/api';
import {
  LayoutDashboard, Terminal, Database, Activity, Settings,
  Search, Zap, Flame, LineChart, Camera, Radio, Gauge, Command, ArrowRight,
} from 'lucide-react';

interface PaletteItem {
  id: string; label: string; description?: string; icon: React.ReactNode;
  action: () => void; category: 'navigate' | 'command';
}

interface CommandPaletteProps { open: boolean; onClose: () => void; }

export default function CommandPalette({ open, onClose }: CommandPaletteProps) {
  const router = useRouter();
  const [query, setQuery] = useState('');
  const [selected, setSelected] = useState(0);
  const [commandResult, setCommandResult] = useState<string | null>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const listRef = useRef<HTMLDivElement>(null);

  const navigate = useCallback((href: string) => { router.push(href); onClose(); }, [router, onClose]);

  const items: PaletteItem[] = [
    { id: 'dashboard', label: 'Dashboard', description: 'Server overview & metrics', icon: <LayoutDashboard size={16} />, action: () => navigate('/'), category: 'navigate' },
    { id: 'console', label: 'Console', description: 'Execute RESP commands', icon: <Terminal size={16} />, action: () => navigate('/console'), category: 'navigate' },
    { id: 'explorer', label: 'Explorer', description: 'Browse & manage keys', icon: <Database size={16} />, action: () => navigate('/explorer'), category: 'navigate' },
    { id: 'monitoring', label: 'Monitoring', description: 'Real-time server metrics', icon: <Activity size={16} />, action: () => navigate('/monitoring'), category: 'navigate' },
    { id: 'hotkeys', label: 'Hot Keys', description: 'Most accessed keys', icon: <Flame size={16} />, action: () => navigate('/hotkeys'), category: 'navigate' },
    { id: 'timeseries', label: 'Time Series', description: 'Time-series data management', icon: <LineChart size={16} />, action: () => navigate('/timeseries'), category: 'navigate' },
    { id: 'snapshots', label: 'Snapshots', description: 'Database snapshots', icon: <Camera size={16} />, action: () => navigate('/snapshots'), category: 'navigate' },
    { id: 'cdc', label: 'CDC', description: 'Change data capture stream', icon: <Radio size={16} />, action: () => navigate('/cdc'), category: 'navigate' },
    { id: 'benchmark', label: 'Benchmark', description: 'Performance benchmarks', icon: <Gauge size={16} />, action: () => navigate('/benchmark'), category: 'navigate' },
    { id: 'settings', label: 'Settings', description: 'Server configuration', icon: <Settings size={16} />, action: () => navigate('/settings'), category: 'navigate' },
    { id: 'cmd-ping', label: 'PING', description: 'Test server connectivity', icon: <Zap size={16} />, action: async () => { const r = await executeCommand('PING'); setCommandResult(String(r.result ?? r.error ?? '')); }, category: 'command' },
    { id: 'cmd-dbsize', label: 'DBSIZE', description: 'Count of database keys', icon: <Command size={16} />, action: async () => { const r = await executeCommand('DBSIZE'); setCommandResult(String(r.result ?? r.error ?? '')); }, category: 'command' },
    { id: 'cmd-info', label: 'INFO', description: 'Server information', icon: <Search size={16} />, action: async () => { const r = await executeCommand('INFO'); setCommandResult(String(r.result ?? r.error ?? '')); }, category: 'command' },
  ];

  const filtered = query.trim()
    ? items.filter((i) => i.label.toLowerCase().includes(query.toLowerCase()) || i.description?.toLowerCase().includes(query.toLowerCase()))
    : items;

  const navItems = filtered.filter(i => i.category === 'navigate');
  const cmdItems = filtered.filter(i => i.category === 'command');

  useEffect(() => { if (open) { setQuery(''); setSelected(0); setCommandResult(null); setTimeout(() => inputRef.current?.focus(), 50); } }, [open]);
  useEffect(() => { setSelected(0); }, [query]);
  useEffect(() => { if (listRef.current) { const el = listRef.current.querySelector(`[data-index="${selected}"]`) as HTMLElement | undefined; el?.scrollIntoView({ block: 'nearest' }); } }, [selected]);

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'ArrowDown') { e.preventDefault(); setSelected((s) => Math.min(s + 1, filtered.length - 1)); }
    else if (e.key === 'ArrowUp') { e.preventDefault(); setSelected((s) => Math.max(s - 1, 0)); }
    else if (e.key === 'Enter' && filtered[selected]) { e.preventDefault(); filtered[selected].action(); }
    else if (e.key === 'Escape') onClose();
  };

  return (
    <AnimatePresence>
      {open && (
        <div className="fixed inset-0 z-50 flex items-start justify-center pt-[12vh]" onClick={onClose}>
          <motion.div className="absolute inset-0"
            initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }}
            style={{ background: 'var(--modal-overlay)', backdropFilter: 'blur(6px)' }} />
          <motion.div className="relative w-full max-w-xl rounded-2xl shadow-2xl overflow-hidden"
            initial={{ opacity: 0, y: -20, scale: 0.96 }} animate={{ opacity: 1, y: 0, scale: 1 }} exit={{ opacity: 0, y: -10, scale: 0.98 }}
            transition={{ type: 'spring', stiffness: 500, damping: 35 }}
            style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border)', boxShadow: 'var(--shadow-xl)' }}
            onClick={(e) => e.stopPropagation()}>
            {/* Search input */}
            <div className="flex items-center gap-3 px-5 py-4" style={{ borderBottom: '1px solid var(--border)' }}>
              <Search size={18} style={{ color: 'var(--text-tertiary)' }} />
              <input ref={inputRef} value={query} onChange={(e) => setQuery(e.target.value)} onKeyDown={handleKeyDown}
                placeholder="Type a command or search…" className="flex-1 bg-transparent text-sm outline-none font-medium"
                style={{ color: 'var(--text-primary)' }} />
              <kbd className="text-[10px] px-1.5 py-0.5 rounded font-mono"
                style={{ background: 'var(--bg-tertiary)', color: 'var(--text-tertiary)' }}>ESC</kbd>
            </div>

            {/* Results */}
            <div ref={listRef} className="max-h-80 overflow-y-auto p-2">
              {filtered.length === 0 && (
                <div className="px-4 py-10 text-center text-sm" style={{ color: 'var(--text-tertiary)' }}>
                  No results for &ldquo;{query}&rdquo;
                </div>
              )}

              {navItems.length > 0 && (
                <>
                  <div className="px-3 py-1.5 text-[10px] font-bold uppercase tracking-widest" style={{ color: 'var(--text-tertiary)' }}>Pages</div>
                  {navItems.map((item) => {
                    const globalIdx = filtered.indexOf(item);
                    return (
                      <button key={item.id} data-index={globalIdx} onClick={() => item.action()}
                        className="w-full flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm text-left transition-colors"
                        style={{ background: globalIdx === selected ? 'var(--brand-muted)' : 'transparent', color: globalIdx === selected ? 'var(--brand)' : 'var(--text-primary)' }}
                        onMouseEnter={() => setSelected(globalIdx)}>
                        <span className="w-8 h-8 rounded-lg flex items-center justify-center flex-shrink-0"
                          style={{ background: 'var(--bg-tertiary)', color: globalIdx === selected ? 'var(--brand)' : 'var(--text-tertiary)' }}>
                          {item.icon}
                        </span>
                        <div className="flex-1 min-w-0">
                          <div className="font-semibold">{item.label}</div>
                          {item.description && <div className="text-xs truncate" style={{ color: 'var(--text-tertiary)' }}>{item.description}</div>}
                        </div>
                        <ArrowRight size={12} className="opacity-0 group-hover:opacity-100" style={{ color: 'var(--text-tertiary)' }} />
                      </button>
                    );
                  })}
                </>
              )}

              {cmdItems.length > 0 && (
                <>
                  <div className="px-3 py-1.5 mt-1 text-[10px] font-bold uppercase tracking-widest" style={{ color: 'var(--text-tertiary)' }}>Commands</div>
                  {cmdItems.map((item) => {
                    const globalIdx = filtered.indexOf(item);
                    return (
                      <button key={item.id} data-index={globalIdx} onClick={() => item.action()}
                        className="w-full flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm text-left transition-colors"
                        style={{ background: globalIdx === selected ? 'var(--brand-muted)' : 'transparent', color: globalIdx === selected ? 'var(--brand)' : 'var(--text-primary)' }}
                        onMouseEnter={() => setSelected(globalIdx)}>
                        <span className="w-8 h-8 rounded-lg flex items-center justify-center flex-shrink-0"
                          style={{ background: 'var(--bg-tertiary)', color: globalIdx === selected ? 'var(--brand)' : 'var(--text-tertiary)' }}>
                          {item.icon}
                        </span>
                        <div className="flex-1 min-w-0">
                          <div className="font-semibold font-mono">{item.label}</div>
                          {item.description && <div className="text-xs truncate" style={{ color: 'var(--text-tertiary)' }}>{item.description}</div>}
                        </div>
                        <span className="text-[10px] px-1.5 py-0.5 rounded font-semibold"
                          style={{ background: 'var(--bg-tertiary)', color: 'var(--text-tertiary)' }}>RUN</span>
                      </button>
                    );
                  })}
                </>
              )}
            </div>

            {/* Command result */}
            {commandResult !== null && (
              <div className="px-5 py-3 text-xs font-mono" style={{ borderTop: '1px solid var(--border)', background: 'var(--bg-inset)', color: 'var(--text-secondary)' }}>
                <span style={{ color: 'var(--success)' }}>→ </span>{commandResult}
              </div>
            )}

            {/* Footer hint */}
            <div className="flex items-center gap-4 px-5 py-2.5 text-[10px]"
              style={{ borderTop: '1px solid var(--border)', color: 'var(--text-tertiary)' }}>
              <span><kbd className="px-1 py-0.5 rounded font-mono" style={{ background: 'var(--bg-tertiary)' }}>↑↓</kbd> Navigate</span>
              <span><kbd className="px-1 py-0.5 rounded font-mono" style={{ background: 'var(--bg-tertiary)' }}>↵</kbd> Select</span>
              <span><kbd className="px-1 py-0.5 rounded font-mono" style={{ background: 'var(--bg-tertiary)' }}>Esc</kbd> Close</span>
            </div>
          </motion.div>
        </div>
      )}
    </AnimatePresence>
  );
}
