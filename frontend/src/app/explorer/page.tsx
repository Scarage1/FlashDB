'use client';

import { useState, useEffect, useCallback } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { Database, Search, RefreshCw, Trash2, Plus, Hash, List, Layers, BarChart3, Type, Copy, Check, ChevronRight, X } from 'lucide-react';
import { executeCommand, getKeys } from '@/lib/api';
import { useToast } from '@/context/ToastContext';
import PageHeader from '@/components/ui/PageHeader';
import EmptyState from '@/components/ui/EmptyState';
import { SkeletonLine } from '@/components/ui/Skeleton';

type DataType = 'string' | 'hash' | 'list' | 'set' | 'zset' | 'unknown';
interface KeyDetail { key: string; type: DataType; ttl: number; value: unknown; }

const TYPE_META: Record<DataType, { icon: React.ReactNode; color: string; label: string }> = {
  string:  { icon: <Type size={14} />, color: '#f59e0b', label: 'String' },
  hash:    { icon: <Hash size={14} />, color: '#0284c7', label: 'Hash' },
  list:    { icon: <List size={14} />, color: '#059669', label: 'List' },
  set:     { icon: <Layers size={14} />, color: '#8b5cf6', label: 'Set' },
  zset:    { icon: <BarChart3 size={14} />, color: '#ef4444', label: 'Sorted Set' },
  unknown: { icon: <Database size={14} />, color: '#71717a', label: 'Unknown' },
};

export default function ExplorerPage() {
  const { showToast } = useToast();
  const [keys, setKeys] = useState<string[]>([]);
  const [filter, setFilter] = useState('');
  const [loading, setLoading] = useState(true);
  const [selectedKey, setSelectedKey] = useState<string | null>(null);
  const [detail, setDetail] = useState<KeyDetail | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);
  const [copiedKey, setCopiedKey] = useState<string | null>(null);
  const [showAdd, setShowAdd] = useState(false);
  const [newKey, setNewKey] = useState('');
  const [newValue, setNewValue] = useState('');

  const fetchKeys = useCallback(async () => { setLoading(true); setKeys(await getKeys()); setLoading(false); }, []);
  useEffect(() => { fetchKeys(); }, [fetchKeys]);

  const filtered = filter ? keys.filter((k) => k.toLowerCase().includes(filter.toLowerCase())) : keys;

  const fetchKeyDetail = useCallback(async (key: string) => {
    setSelectedKey(key); setDetailLoading(true);
    const typeRes = await executeCommand(`TYPE ${key}`);
    const rawType = String(typeRes.result ?? 'unknown').toLowerCase();
    let type: DataType = 'unknown';
    if (['string','hash','list','set','zset'].includes(rawType)) type = rawType as DataType;
    const ttlRes = await executeCommand(`TTL ${key}`);
    const ttl = Number(ttlRes.result ?? -1);
    let value: unknown = null;
    switch (type) {
      case 'string': { const r = await executeCommand(`GET ${key}`); value = r.result; break; }
      case 'hash': { const r = await executeCommand(`HGETALL ${key}`); value = r.result; break; }
      case 'list': { const r = await executeCommand(`LRANGE ${key} 0 -1`); value = r.result; break; }
      case 'set': { const r = await executeCommand(`SMEMBERS ${key}`); value = r.result; break; }
      case 'zset': { const r = await executeCommand(`ZRANGE ${key} 0 -1`); value = r.result; break; }
      default: { const r = await executeCommand(`GET ${key}`); value = r.result; }
    }
    setDetail({ key, type, ttl, value }); setDetailLoading(false);
  }, []);

  const deleteKey = async (key: string) => { await executeCommand(`DEL ${key}`); showToast(`Deleted ${key}`, 'success'); setSelectedKey(null); setDetail(null); fetchKeys(); };
  const addKey = async () => { if (!newKey.trim()) return; await executeCommand(`SET ${newKey.trim()} ${newValue.trim() || '""'}`); showToast(`Created ${newKey.trim()}`, 'success'); setShowAdd(false); setNewKey(''); setNewValue(''); fetchKeys(); };

  const renderValue = (d: KeyDetail) => {
    if (d.type === 'string') return <pre className="text-sm font-mono whitespace-pre-wrap p-4 rounded-lg" style={{ background: 'var(--bg-inset)', color: 'var(--text-primary)' }}>{String(d.value ?? '(nil)')}</pre>;
    if (d.type === 'hash') {
      const entries = Array.isArray(d.value) ? d.value : [];
      const pairs: [string, string][] = [];
      for (let i = 0; i < entries.length; i += 2) pairs.push([String(entries[i]), String(entries[i + 1] ?? '')]);
      return (
        <div className="rounded-lg overflow-hidden" style={{ border: '1px solid var(--border)' }}>
          <table className="w-full text-sm">
            <thead><tr style={{ background: 'var(--bg-secondary)' }}><th className="text-left py-2.5 px-4 text-xs font-semibold" style={{ color: 'var(--text-tertiary)' }}>Field</th><th className="text-left py-2.5 px-4 text-xs font-semibold" style={{ color: 'var(--text-tertiary)' }}>Value</th></tr></thead>
            <tbody>{pairs.map(([f, v], i) => (<tr key={i} style={{ borderTop: '1px solid var(--border)' }}><td className="py-2.5 px-4 font-mono text-xs" style={{ color: 'var(--brand)' }}>{f}</td><td className="py-2.5 px-4 font-mono text-xs" style={{ color: 'var(--text-primary)' }}>{v}</td></tr>))}</tbody>
          </table>
        </div>
      );
    }
    const items = Array.isArray(d.value) ? d.value : [];
    return (
      <div className="space-y-1">
        {!items.length && <p className="text-sm py-8 text-center" style={{ color: 'var(--text-tertiary)' }}>(empty)</p>}
        {items.map((itm, i) => (
          <div key={i} className="flex items-center gap-3 px-4 py-2 rounded-lg text-xs font-mono" style={{ background: 'var(--bg-inset)', color: 'var(--text-primary)' }}>
            <span className="w-6 text-right" style={{ color: 'var(--text-tertiary)' }}>{i + 1}</span>
            <span>{String(itm)}</span>
          </div>
        ))}
      </div>
    );
  };

  return (
    <motion.div className="max-w-7xl mx-auto px-4 sm:px-6 py-8" style={{ height: 'calc(100vh - 64px)' }}
      initial={{ opacity: 0 }} animate={{ opacity: 1 }} transition={{ duration: 0.3 }}>
      <PageHeader title="Explorer" description={`${keys.length} key${keys.length !== 1 ? 's' : ''} in database`}
        actions={<>
          <button onClick={() => setShowAdd(true)} className="btn-primary !px-3 !py-1.5 !text-xs"><Plus size={14} /> Add Key</button>
          <button onClick={fetchKeys} className="btn-secondary !px-3 !py-1.5 !text-xs"><RefreshCw size={14} className={loading ? 'animate-spin' : ''} /> Refresh</button>
        </>}
      />

      <div className="flex gap-4" style={{ height: 'calc(100% - 90px)' }}>
        {/* Key list */}
        <div className="w-72 lg:w-80 flex-shrink-0 flex flex-col card overflow-hidden">
          <div className="p-3" style={{ borderBottom: '1px solid var(--border)' }}>
            <div className="flex items-center gap-2 px-3 py-2 rounded-lg" style={{ background: 'var(--bg-inset)', border: '1px solid var(--border)' }}>
              <Search size={14} style={{ color: 'var(--text-tertiary)' }} />
              <input value={filter} onChange={(e) => setFilter(e.target.value)} placeholder="Filter keys…" className="flex-1 text-xs bg-transparent outline-none" style={{ color: 'var(--text-primary)' }} />
              {filter && <button onClick={() => setFilter('')}><X size={12} style={{ color: 'var(--text-tertiary)' }} /></button>}
            </div>
          </div>
          <div className="flex-1 overflow-y-auto p-1.5">
            {loading ? Array.from({ length: 8 }).map((_, i) => <div key={i} className="px-3 py-2"><SkeletonLine width={`${60 + Math.random() * 30}%`} height="14px" /></div>) :
              !filtered.length ? (
                <EmptyState icon={<Database size={24} />} title={keys.length ? 'No match' : 'No keys yet'} description={keys.length ? 'Try a different filter' : 'Add your first key to get started'} />
              ) : filtered.map((key) => (
                <button key={key} onClick={() => fetchKeyDetail(key)}
                  className="w-full flex items-center gap-2 px-3 py-2 rounded-lg text-xs font-mono text-left transition-colors group"
                  style={{ background: selectedKey === key ? 'var(--brand-muted)' : 'transparent', color: selectedKey === key ? 'var(--brand)' : 'var(--text-primary)' }}
                  onMouseEnter={(e) => { if (selectedKey !== key) e.currentTarget.style.background = 'var(--bg-secondary)'; }}
                  onMouseLeave={(e) => { if (selectedKey !== key) e.currentTarget.style.background = 'transparent'; }}>
                  <ChevronRight size={11} style={{ color: 'var(--text-tertiary)' }} />
                  <span className="truncate flex-1">{key}</span>
                  <button onClick={(e) => { e.stopPropagation(); navigator.clipboard.writeText(key); setCopiedKey(key); setTimeout(() => setCopiedKey(null), 1500); }}
                    className="opacity-0 group-hover:opacity-100 transition-opacity" style={{ color: 'var(--text-tertiary)' }}>
                    {copiedKey === key ? <Check size={11} /> : <Copy size={11} />}
                  </button>
                </button>
              ))
            }
          </div>
        </div>

        {/* Detail */}
        <div className="flex-1 card overflow-y-auto">
          {detailLoading ? (
            <div className="p-6 space-y-4"><SkeletonLine width="30%" height="24px" /><SkeletonLine width="100%" height="120px" /></div>
          ) : !detail ? (
            <EmptyState icon={<Database size={28} />} title="Select a key" description="Choose a key from the list to view its type, TTL, and value." />
          ) : (
            <div className="p-6 space-y-5">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-3">
                  <span className="flex items-center gap-1.5 px-2.5 py-1 rounded-lg text-xs font-semibold"
                    style={{ background: `color-mix(in srgb, ${TYPE_META[detail.type].color} 10%, transparent)`, color: TYPE_META[detail.type].color }}>
                    {TYPE_META[detail.type].icon} {TYPE_META[detail.type].label}
                  </span>
                  <h2 className="text-lg font-mono font-bold" style={{ color: 'var(--text-primary)' }}>{detail.key}</h2>
                </div>
                <div className="flex items-center gap-2">
                  {detail.ttl >= 0 && <span className="text-xs px-2.5 py-1 rounded-lg" style={{ background: 'var(--bg-tertiary)', color: 'var(--text-secondary)' }}>TTL: {detail.ttl}s</span>}
                  <button onClick={() => deleteKey(detail.key)} className="btn-secondary !px-2.5 !py-1 !text-xs" style={{ color: 'var(--error)' }}><Trash2 size={12} /> Delete</button>
                </div>
              </div>
              <div style={{ borderTop: '1px solid var(--border)' }} className="pt-5">{renderValue(detail)}</div>
            </div>
          )}
        </div>
      </div>

      {/* Add modal */}
      <AnimatePresence>
        {showAdd && (
          <div className="fixed inset-0 z-50 flex items-center justify-center" onClick={() => setShowAdd(false)}>
            <motion.div className="absolute inset-0" initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }}
              style={{ background: 'var(--modal-overlay)', backdropFilter: 'blur(4px)' }} />
            <motion.div className="relative w-full max-w-md rounded-xl p-6"
              initial={{ opacity: 0, scale: 0.95 }} animate={{ opacity: 1, scale: 1 }} exit={{ opacity: 0, scale: 0.95 }}
              style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border)', boxShadow: 'var(--shadow-xl)' }} onClick={(e) => e.stopPropagation()}>
              <h3 className="font-display text-lg font-bold mb-4" style={{ color: 'var(--text-primary)' }}>Add Key</h3>
              <div className="space-y-3">
                <div><label className="text-xs font-semibold mb-1.5 block" style={{ color: 'var(--text-secondary)' }}>Key</label>
                  <input value={newKey} onChange={(e) => setNewKey(e.target.value)} className="input-field" placeholder="my:key" autoFocus /></div>
                <div><label className="text-xs font-semibold mb-1.5 block" style={{ color: 'var(--text-secondary)' }}>Value</label>
                  <input value={newValue} onChange={(e) => setNewValue(e.target.value)} className="input-field" placeholder="hello world" onKeyDown={(e) => e.key === 'Enter' && addKey()} /></div>
              </div>
              <div className="flex justify-end gap-2 mt-5">
                <button onClick={() => setShowAdd(false)} className="btn-secondary !text-xs">Cancel</button>
                <button onClick={addKey} disabled={!newKey.trim()} className="btn-primary !text-xs disabled:opacity-40">Create</button>
              </div>
            </motion.div>
          </div>
        )}
      </AnimatePresence>
    </motion.div>
  );
}
