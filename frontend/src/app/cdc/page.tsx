'use client';

import { useState, useEffect, useCallback, useRef } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { Radio, Pause, Play, RefreshCw, Trash2, Filter, ArrowRight } from 'lucide-react';
import { getCDCEvents, subscribeCDC, type CDCEvent, type CDCStats } from '@/lib/api';
import PageHeader from '@/components/ui/PageHeader';
import StatCard from '@/components/ui/StatCard';
import Badge from '@/components/ui/Badge';
import EmptyState from '@/components/ui/EmptyState';
import { SkeletonCard, SkeletonTable } from '@/components/ui/Skeleton';

const container = { hidden: {}, show: { transition: { staggerChildren: 0.05 } } };
const item = { hidden: { opacity: 0, y: 10 }, show: { opacity: 1, y: 0 } };

const OP_COLORS: Record<string, string> = { SET: '#22c55e', DEL: '#ef4444', EXPIRE: '#f59e0b', HSET: '#0284c7', LPUSH: '#8b5cf6', RPUSH: '#8b5cf6', SADD: '#ec4899', ZADD: '#06b6d4' };
function opColor(op: string) { return OP_COLORS[op.toUpperCase()] || '#71717a'; }

export default function CDCPage() {
  const [events, setEvents] = useState<CDCEvent[]>([]);
  const [stats, setStats] = useState<CDCStats | null>(null);
  const [loading, setLoading] = useState(true);
  const [streaming, setStreaming] = useState(false);
  const [filterOp, setFilterOp] = useState('');
  const closeRef = useRef<(() => void) | null>(null);

  const fetchEvents = useCallback(async () => {
    setLoading(true);
    const data = await getCDCEvents();
    setEvents(data.events || []);
    setStats(data.stats || null);
    setLoading(false);
  }, []);

  useEffect(() => { fetchEvents(); }, [fetchEvents]);

  const startStreaming = () => {
    if (closeRef.current) return;
    setStreaming(true);
    const unsub = subscribeCDC((evt: CDCEvent) => {
      setEvents((prev) => [evt, ...prev].slice(0, 500));
    });
    closeRef.current = unsub;
  };

  const stopStreaming = () => {
    closeRef.current?.();
    closeRef.current = null;
    setStreaming(false);
  };

  useEffect(() => () => { closeRef.current?.(); }, []);

  const filtered = filterOp ? events.filter((e) => e.op.toUpperCase() === filterOp.toUpperCase()) : events;
  const uniqueOps = [...new Set(events.map((e) => e.op.toUpperCase()))];

  return (
    <motion.div className="max-w-6xl mx-auto px-4 sm:px-6 py-8" initial={{ opacity: 0, y: 12 }} animate={{ opacity: 1, y: 0 }} transition={{ duration: 0.35 }}>
      <PageHeader title="Change Data Capture" description="Real-time event stream from database mutations"
        badge={streaming ? <Badge variant="soft" color="#22c55e">● Streaming</Badge> : <Badge variant="outline" color="var(--text-tertiary)">Paused</Badge>}
        actions={<>
          {streaming ? (
            <button onClick={stopStreaming} className="btn-secondary !px-3 !py-1.5 !text-xs" style={{ color: 'var(--error)' }}><Pause size={14} /> Stop</button>
          ) : (
            <button onClick={startStreaming} className="btn-primary !px-3 !py-1.5 !text-xs"><Play size={14} /> Stream</button>
          )}
          <button onClick={fetchEvents} className="btn-secondary !px-3 !py-1.5 !text-xs"><RefreshCw size={14} className={loading ? 'animate-spin' : ''} /></button>
          <button onClick={() => setEvents([])} className="btn-secondary !px-3 !py-1.5 !text-xs" style={{ color: 'var(--error)' }}><Trash2 size={14} /></button>
        </>}
      />

      {/* Stats */}
      {loading ? (
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-4 mb-8">{Array.from({ length: 4 }).map((_, i) => <SkeletonCard key={i} />)}</div>
      ) : stats ? (
        <motion.div className="grid grid-cols-2 sm:grid-cols-4 gap-4 mb-8" variants={container} initial="hidden" animate="show">
          <motion.div variants={item}><StatCard icon={<Radio size={16} />} label="Total Events" value={stats.total_events.toLocaleString()} color="#f59e0b" /></motion.div>
          <motion.div variants={item}><StatCard icon={<ArrowRight size={16} />} label="Buffer Size" value={`${stats.buffer_size} / ${stats.buffer_cap}`} color="#0284c7" /></motion.div>
          <motion.div variants={item}><StatCard icon={<Radio size={16} />} label="Subscribers" value={String(stats.subscribers)} color="#22c55e" /></motion.div>
          <motion.div variants={item}><StatCard icon={<Filter size={16} />} label="Displayed" value={`${filtered.length} events`} color="#8b5cf6" /></motion.div>
        </motion.div>
      ) : null}

      {/* Filter */}
      {uniqueOps.length > 0 && (
        <div className="flex items-center gap-2 mb-5 flex-wrap">
          <span className="text-xs font-semibold" style={{ color: 'var(--text-tertiary)' }}>Filter:</span>
          <button onClick={() => setFilterOp('')} className="px-2.5 py-1 rounded-lg text-xs font-mono transition-colors"
            style={{ background: !filterOp ? 'var(--brand-muted)' : 'var(--bg-secondary)', color: !filterOp ? 'var(--brand)' : 'var(--text-secondary)' }}>All</button>
          {uniqueOps.map((op) => (
            <button key={op} onClick={() => setFilterOp(filterOp === op ? '' : op)}
              className="px-2.5 py-1 rounded-lg text-xs font-mono transition-colors"
              style={{ background: filterOp === op ? `color-mix(in srgb, ${opColor(op)} 15%, transparent)` : 'var(--bg-secondary)',
                color: filterOp === op ? opColor(op) : 'var(--text-secondary)' }}>{op}</button>
          ))}
        </div>
      )}

      {/* Event Feed */}
      <div className="card overflow-hidden">
        <div className="flex items-center justify-between px-5 py-3" style={{ borderBottom: '1px solid var(--border)' }}>
          <div className="flex items-center gap-2.5">
            <Radio size={16} style={{ color: streaming ? '#22c55e' : 'var(--text-tertiary)' }} className={streaming ? 'animate-pulse' : ''} />
            <h3 className="text-sm font-display font-bold" style={{ color: 'var(--text-primary)' }}>Event Feed</h3>
          </div>
          <Badge variant="outline" color="var(--text-tertiary)">{filtered.length}</Badge>
        </div>

        {loading ? <SkeletonTable rows={6} /> : !filtered.length ? (
          <EmptyState icon={<Radio size={28} />} title="No events captured"
            description={streaming ? 'Waiting for database mutations…' : 'Start streaming or perform some database operations to capture events.'} />
        ) : (
          <div className="divide-y" style={{ borderColor: 'var(--border)', maxHeight: 'calc(100vh - 460px)', overflowY: 'auto' }}>
            <AnimatePresence initial={false}>
              {filtered.slice(0, 200).map((evt) => (
                <motion.div key={evt.id} className="flex items-center gap-4 px-5 py-3 transition-colors text-sm"
                  initial={{ opacity: 0, height: 0 }} animate={{ opacity: 1, height: 'auto' }} exit={{ opacity: 0 }}
                  onMouseEnter={(e) => e.currentTarget.style.background = 'var(--bg-secondary)'}
                  onMouseLeave={(e) => e.currentTarget.style.background = 'transparent'}>
                  <Badge variant="soft" color={opColor(evt.op)}>{evt.op.toUpperCase()}</Badge>
                  <span className="font-mono text-xs font-semibold" style={{ color: 'var(--text-primary)' }}>{evt.key}</span>
                  {evt.value && <span className="text-xs font-mono truncate max-w-[200px]" style={{ color: 'var(--text-tertiary)' }}>{evt.value}</span>}
                  <span className="ml-auto text-xs tabular-nums" style={{ color: 'var(--text-tertiary)' }}>
                    {evt.ts ? new Date(evt.ts).toLocaleTimeString() : '—'}
                  </span>
                </motion.div>
              ))}
            </AnimatePresence>
          </div>
        )}
      </div>
    </motion.div>
  );
}
