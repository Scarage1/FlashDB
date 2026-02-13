'use client';

import { useToast } from '@/context/ToastContext';
import { CheckCircle, XCircle, Info, X } from 'lucide-react';
import { motion, AnimatePresence } from 'framer-motion';

export default function Toast() {
  const { message, type, show, hideToast } = useToast();

  const config = {
    success: { icon: <CheckCircle size={16} />, color: 'var(--success)', bg: 'var(--success-muted)' },
    error:   { icon: <XCircle size={16} />, color: 'var(--error)', bg: 'var(--error-muted)' },
    info:    { icon: <Info size={16} />, color: 'var(--info)', bg: 'var(--info-muted)' },
  };

  const c = config[type];

  return (
    <AnimatePresence>
      {show && (
        <motion.div
          initial={{ y: 24, opacity: 0, scale: 0.95 }}
          animate={{ y: 0, opacity: 1, scale: 1 }}
          exit={{ y: 24, opacity: 0, scale: 0.95 }}
          transition={{ type: 'spring', stiffness: 400, damping: 30 }}
          className="fixed bottom-6 right-6 flex items-center gap-3 px-4 py-3 rounded-xl z-50 max-w-sm"
          style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border)', boxShadow: 'var(--shadow-lg)' }}>
          <span className="flex-shrink-0 w-7 h-7 rounded-lg flex items-center justify-center"
            style={{ background: c.bg, color: c.color }}>
            {c.icon}
          </span>
          <span className="text-sm font-medium flex-1" style={{ color: 'var(--text-primary)' }}>{message}</span>
          <button onClick={hideToast} className="flex-shrink-0 p-1 rounded-md hover:bg-[var(--bg-tertiary)] transition-colors"
            style={{ color: 'var(--text-tertiary)' }}>
            <X size={14} />
          </button>
        </motion.div>
      )}
    </AnimatePresence>
  );
}
