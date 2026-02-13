'use client';

import { useState, useEffect, useCallback } from 'react';
import { motion } from 'framer-motion';
import { Flame, RefreshCw, TrendingUp, Hash, BarChart3, Search } from 'lucide-react';
import { getHotKeys, type HotKeyEntry } from '@/lib/api';
import PageHeader from '@/components/ui/PageHeader';
import StatCard from '@/components/ui/StatCard';
import Badge from '@/components/ui/Badge';
import EmptyState from '@/components/ui/EmptyState';
import { SkeletonCard, SkeletonTable } from '@/components/ui/Skeleton';

const container = { hidden: {}, show: { transition: { staggerChildren: 0.05 } } };
const item = { hidden: { opacity: 0, y: 10 }, show: { opacity: 1, y: 0 } };

export default function HotKeysPage() {
  const [keys, setKeys] = useState<HotKeyEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [topN, setTopN] = useState(20);
  const [filter, setFilter] = useState('');

  const fetchKeys = useCallback(async () => { setLoading(true); setKeys(await getHotKeys(topN)); setLoading(false); }, [topN]);
  useEffect(() => { fetchKeys(); }, [fetchKeys]);

  const maxCount = Math.max(1, ...keys.map((k) => k.count));
  const filtered = filter ? keys.filter((k) => k.key.toLowerCase().includes(filter.toLowerCase())) : keys;
  const totalAccess = keys.reduce((s, k) => s + k.count, 0);
  const topKey = keys[0];

  return (
    <motion.div className="max-w-5xl mx-auto px-4 sm:px-6 py-8" initial={{ opacity: 0, y: 12 }} animate={{ opacity: 1, y: 0 }} transition={{ duration: 0.35 }}>
      <PageHeader title="Hot Keys" description="Most frequently accessed keys ranked by access count"
        badge={<Badge variant="soft" color="#f59e0b"><Flame size={12} /> Access Tracker</Badge>}
        actions={<>
          <select value={topN} onChange={(e) => setTopN(Number(e.target.value))} className="input-field !text-xs !w-auto !py-1.5">
            <option value={10}>Top 10</option><option value={20}>Top 20</option><option value={50}>Top 50</option>
          </select>
          <button onClick={fetchKeys} className="btn-secondary !px-3 !py-1.5 !text-xs"><RefreshCw size={14} className={loading ? 'animate-spin' : ''} /></button>
        </>} />

      {/* Stats */}
      {loading ? (
        <div className="grid grid-cols-3 gap-4 mb-8">{Array.from({ length: 3 }).map((_, i) => <SkeletonCard key={i} />)}</div>
      ) : (
        <motion.div className="grid grid-cols-3 gap-4 mb-8" variants={container} initial="hidden" animate="show">
          <motion.div variants={item}><StatCard icon={<Hash size={18} />} label="Tracked Keys" value={String(keys.length)} color="#f59e0b" /></motion.div>
          <motion.div variants={item}><StatCard icon={<TrendingUp size={18} />} label="Total Accesses" value={totalAccess.toLocaleString()} color="#0284c7" /></motion.div>
          <motion.div variants={item}><StatCard icon={<Flame size={18} />} label="Hottest Key" value={topKey?.key || '—'} color="#ef4444" /></motion.div>
        </motion.div>
      )}

      {/* Table */}
      <div className="card overflow-hidden">
        <div className="flex items-center justify-between px-5 py-3" style={{ borderBottom: '1px solid var(--border)' }}>
          <div className="flex items-center gap-2.5">
            <BarChart3 size={16} style={{ color: 'var(--brand)' }} />
            <h3 className="text-sm font-display font-bold" style={{ color: 'var(--text-primary)' }}>Access Distribution</h3>
          </div>
          <div className="flex items-center gap-2 px-3 py-1.5 rounded-lg" style={{ background: 'var(--bg-inset)', border: '1px solid var(--border)' }}>
            <Search size={12} style={{ color: 'var(--text-tertiary)' }} />
            <input value={filter} onChange={(e) => setFilter(e.target.value)} placeholder="Filter…" className="text-xs bg-transparent outline-none w-28" style={{ color: 'var(--text-primary)' }} />
          </div>
        </div>

        {loading ? <SkeletonTable rows={6} /> : !filtered.length ? (
          <EmptyState icon={<Flame size={24} />} title="No hot keys" description="Access some keys in your database first, then they'll appear here ranked by frequency." />
        ) : (
          <div className="p-4 space-y-2">
            {filtered.map((k, i) => {
              const pct = Math.round((k.count / maxCount) * 100);
              return (
                <motion.div key={k.key} className="flex items-center gap-4 px-4 py-3 rounded-lg transition-colors"
                  initial={{ opacity: 0, x: -12 }} animate={{ opacity: 1, x: 0 }} transition={{ delay: i * 0.03 }}
                  style={{ background: 'var(--bg-secondary)' }}
                  onMouseEnter={(e) => e.currentTarget.style.background = 'var(--bg-tertiary)'}
                  onMouseLeave={(e) => e.currentTarget.style.background = 'var(--bg-secondary)'}>
                  <span className="text-xs font-mono w-6 text-right" style={{ color: 'var(--text-tertiary)' }}>#{i + 1}</span>
                  <span className="font-mono text-sm font-semibold flex-shrink-0 w-48 truncate" style={{ color: 'var(--text-primary)' }}>{k.key}</span>
                  <div className="flex-1 h-2.5 rounded-full overflow-hidden" style={{ background: 'var(--bg-inset)' }}>
                    <motion.div className="h-full rounded-full" initial={{ width: 0 }} animate={{ width: `${pct}%` }}
                      transition={{ duration: 0.6, delay: i * 0.03 }}
                      style={{ background: `linear-gradient(90deg, var(--brand), ${i < 3 ? '#ef4444' : 'var(--brand)'})` }} />
                  </div>
                  <span className="text-xs font-mono font-bold w-20 text-right" style={{ color: i < 3 ? '#ef4444' : 'var(--brand)' }}>{k.count.toLocaleString()}</span>
                </motion.div>
              );
            })}
          </div>
        )}
      </div>
    </motion.div>
  );
}
