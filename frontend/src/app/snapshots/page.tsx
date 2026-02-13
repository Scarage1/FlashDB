'use client';

import { useState, useEffect, useCallback } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { Camera, Plus, RotateCcw, Trash2, HardDrive, Clock, FileText, RefreshCw } from 'lucide-react';
import { listSnapshots, createSnapshot, restoreSnapshot, deleteSnapshot, type SnapshotMeta } from '@/lib/api';
import { useToast } from '@/context/ToastContext';
import PageHeader from '@/components/ui/PageHeader';
import Badge from '@/components/ui/Badge';
import EmptyState from '@/components/ui/EmptyState';
import { SkeletonTable } from '@/components/ui/Skeleton';

function fmtSize(b: number) { if (b >= 1e9) return `${(b / 1e9).toFixed(2)} GB`; if (b >= 1e6) return `${(b / 1e6).toFixed(1)} MB`; if (b >= 1e3) return `${(b / 1e3).toFixed(1)} KB`; return `${b} B`; }
function fmtTime(iso: string) { if (!iso) return '—'; return new Date(iso).toLocaleString(); }

export default function SnapshotsPage() {
  const { showToast } = useToast();
  const [snapshots, setSnapshots] = useState<SnapshotMeta[]>([]);
  const [loading, setLoading] = useState(true);
  const [creating, setCreating] = useState(false);
  const [showCreate, setShowCreate] = useState(false);
  const [newName, setNewName] = useState('');
  const [confirmDelete, setConfirmDelete] = useState<string | null>(null);
  const [confirmRestore, setConfirmRestore] = useState<string | null>(null);

  const fetchSnapshots = useCallback(async () => { setLoading(true); setSnapshots(await listSnapshots()); setLoading(false); }, []);
  useEffect(() => { fetchSnapshots(); }, [fetchSnapshots]);

  const handleCreate = async () => {
    const name = newName.trim() || `snapshot-${Date.now()}`;
    setCreating(true);
    await createSnapshot(name);
    showToast(`Snapshot "${name}" created`, 'success');
    setShowCreate(false); setNewName(''); setCreating(false);
    fetchSnapshots();
  };

  const handleRestore = async (id: string) => {
    await restoreSnapshot(id);
    showToast(`Restored snapshot "${id}"`, 'success');
    setConfirmRestore(null);
  };

  const handleDelete = async (id: string) => {
    await deleteSnapshot(id);
    showToast(`Deleted snapshot "${id}"`, 'success');
    setConfirmDelete(null);
    fetchSnapshots();
  };

  const totalSize = snapshots.reduce((s, sn) => s + sn.size_bytes, 0);

  return (
    <motion.div className="max-w-5xl mx-auto px-4 sm:px-6 py-8" initial={{ opacity: 0, y: 12 }} animate={{ opacity: 1, y: 0 }} transition={{ duration: 0.35 }}>
      <PageHeader title="Snapshots" description="Create and manage database snapshots"
        badge={<Badge variant="soft" color="#8b5cf6"><Camera size={12} /> Persistence</Badge>}
        actions={<>
          <button onClick={() => setShowCreate(true)} className="btn-primary !px-3 !py-1.5 !text-xs"><Plus size={14} /> Create Snapshot</button>
          <button onClick={fetchSnapshots} className="btn-secondary !px-3 !py-1.5 !text-xs"><RefreshCw size={14} className={loading ? 'animate-spin' : ''} /></button>
        </>}
      />

      {/* Summary */}
      <div className="grid grid-cols-3 gap-4 mb-6">
        <div className="card p-4 flex items-center gap-3">
          <div className="w-9 h-9 rounded-lg flex items-center justify-center" style={{ background: 'color-mix(in srgb, #8b5cf6 10%, transparent)' }}>
            <Camera size={18} style={{ color: '#8b5cf6' }} />
          </div>
          <div><p className="text-xs" style={{ color: 'var(--text-tertiary)' }}>Snapshots</p><p className="font-display font-bold text-lg" style={{ color: 'var(--text-primary)' }}>{snapshots.length}</p></div>
        </div>
        <div className="card p-4 flex items-center gap-3">
          <div className="w-9 h-9 rounded-lg flex items-center justify-center" style={{ background: 'color-mix(in srgb, #0284c7 10%, transparent)' }}>
            <HardDrive size={18} style={{ color: '#0284c7' }} />
          </div>
          <div><p className="text-xs" style={{ color: 'var(--text-tertiary)' }}>Total Size</p><p className="font-display font-bold text-lg" style={{ color: 'var(--text-primary)' }}>{fmtSize(totalSize)}</p></div>
        </div>
        <div className="card p-4 flex items-center gap-3">
          <div className="w-9 h-9 rounded-lg flex items-center justify-center" style={{ background: 'color-mix(in srgb, #22c55e 10%, transparent)' }}>
            <Clock size={18} style={{ color: '#22c55e' }} />
          </div>
          <div><p className="text-xs" style={{ color: 'var(--text-tertiary)' }}>Latest</p><p className="font-display font-bold text-sm" style={{ color: 'var(--text-primary)' }}>{snapshots.length ? fmtTime(snapshots[0].created_at) : '—'}</p></div>
        </div>
      </div>

      {/* Table */}
      <div className="card overflow-hidden">
        <div className="flex items-center gap-2.5 px-5 py-3" style={{ borderBottom: '1px solid var(--border)' }}>
          <Camera size={16} style={{ color: 'var(--brand)' }} />
          <h3 className="text-sm font-display font-bold" style={{ color: 'var(--text-primary)' }}>All Snapshots</h3>
        </div>

        {loading ? <SkeletonTable rows={4} /> : !snapshots.length ? (
          <EmptyState icon={<Camera size={28} />} title="No snapshots" description="Create your first snapshot to persist the current database state."
            action={<button onClick={() => setShowCreate(true)} className="btn-primary !text-xs"><Plus size={14} /> Create Snapshot</button>} />
        ) : (
          <div className="divide-y" style={{ borderColor: 'var(--border)' }}>
            {snapshots.map((snap) => (
              <div key={snap.id} className="flex items-center gap-4 px-5 py-4 transition-colors"
                onMouseEnter={(e) => e.currentTarget.style.background = 'var(--bg-secondary)'}
                onMouseLeave={(e) => e.currentTarget.style.background = 'transparent'}>
                <div className="w-9 h-9 rounded-lg flex items-center justify-center flex-shrink-0" style={{ background: 'color-mix(in srgb, #8b5cf6 8%, transparent)' }}>
                  <FileText size={16} style={{ color: '#8b5cf6' }} />
                </div>
                <div className="flex-1 min-w-0">
                  <p className="text-sm font-mono font-semibold truncate" style={{ color: 'var(--text-primary)' }}>{snap.id}</p>
                  <div className="flex items-center gap-3 mt-0.5 text-xs" style={{ color: 'var(--text-tertiary)' }}>
                    <span>{fmtTime(snap.created_at)}</span>
                    <span>·</span>
                    <span>{fmtSize(snap.size_bytes)}</span>
                    {snap.file_path && <><span>·</span><span className="truncate max-w-[200px]">{snap.file_path}</span></>}
                  </div>
                </div>
                <div className="flex items-center gap-2 flex-shrink-0">
                  {confirmRestore === snap.id ? (
                    <div className="flex items-center gap-1.5">
                      <button onClick={() => handleRestore(snap.id)} className="btn-primary !px-2.5 !py-1 !text-xs">Confirm</button>
                      <button onClick={() => setConfirmRestore(null)} className="btn-secondary !px-2.5 !py-1 !text-xs">Cancel</button>
                    </div>
                  ) : (
                    <button onClick={() => setConfirmRestore(snap.id)} className="btn-secondary !px-2.5 !py-1 !text-xs"><RotateCcw size={12} /> Restore</button>
                  )}
                  {confirmDelete === snap.id ? (
                    <div className="flex items-center gap-1.5">
                      <button onClick={() => handleDelete(snap.id)} className="btn-primary !px-2.5 !py-1 !text-xs" style={{ background: 'var(--error)' }}>Delete</button>
                      <button onClick={() => setConfirmDelete(null)} className="btn-secondary !px-2.5 !py-1 !text-xs">Cancel</button>
                    </div>
                  ) : (
                    <button onClick={() => setConfirmDelete(snap.id)} className="btn-secondary !px-2.5 !py-1 !text-xs" style={{ color: 'var(--error)' }}><Trash2 size={12} /></button>
                  )}
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Create Modal */}
      <AnimatePresence>
        {showCreate && (
          <div className="fixed inset-0 z-50 flex items-center justify-center" onClick={() => setShowCreate(false)}>
            <motion.div className="absolute inset-0" initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }}
              style={{ background: 'var(--modal-overlay)', backdropFilter: 'blur(4px)' }} />
            <motion.div className="relative w-full max-w-md rounded-xl p-6"
              initial={{ opacity: 0, scale: 0.95 }} animate={{ opacity: 1, scale: 1 }} exit={{ opacity: 0, scale: 0.95 }}
              style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border)', boxShadow: 'var(--shadow-xl)' }} onClick={(e) => e.stopPropagation()}>
              <h3 className="font-display text-lg font-bold mb-4" style={{ color: 'var(--text-primary)' }}>Create Snapshot</h3>
              <div>
                <label className="text-xs font-semibold mb-1.5 block" style={{ color: 'var(--text-secondary)' }}>Name (optional)</label>
                <input value={newName} onChange={(e) => setNewName(e.target.value)} className="input-field" placeholder="my-snapshot"
                  onKeyDown={(e) => e.key === 'Enter' && handleCreate()} autoFocus />
              </div>
              <div className="flex justify-end gap-2 mt-5">
                <button onClick={() => setShowCreate(false)} className="btn-secondary !text-xs">Cancel</button>
                <button onClick={handleCreate} disabled={creating} className="btn-primary !text-xs disabled:opacity-50">
                  {creating ? 'Creating…' : 'Create'}
                </button>
              </div>
            </motion.div>
          </div>
        )}
      </AnimatePresence>
    </motion.div>
  );
}
