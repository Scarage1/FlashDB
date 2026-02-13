'use client';

import { useState, useEffect, useCallback, ReactNode } from 'react';
import Navbar from '@/components/Navbar';
import CommandPalette from '@/components/CommandPalette';
import Toast from '@/components/Toast';
import { ToastProvider } from '@/context/ToastContext';
import { ThemeProvider } from '@/context/ThemeContext';

export default function AppShell({ children }: { children: ReactNode }) {
  const [paletteOpen, setPaletteOpen] = useState(false);
  const openPalette = useCallback(() => setPaletteOpen(true), []);
  const closePalette = useCallback(() => setPaletteOpen(false), []);

  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
        e.preventDefault();
        setPaletteOpen((prev) => !prev);
      }
    };
    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
  }, []);

  return (
    <ThemeProvider>
      <ToastProvider>
        <div className="min-h-screen flex flex-col" style={{ background: 'var(--bg-primary)' }}>
          <Navbar onCommandPalette={openPalette} />
          <main className="flex-1">{children}</main>
          <CommandPalette open={paletteOpen} onClose={closePalette} />
          <Toast />
        </div>
      </ToastProvider>
    </ThemeProvider>
  );
}
