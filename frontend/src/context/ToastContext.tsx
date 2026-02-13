'use client';

import { createContext, useContext, useState, useCallback, ReactNode } from 'react';

interface ToastContextType {
  message: string;
  type: 'success' | 'error' | 'info';
  show: boolean;
  showToast: (message: string, type?: 'success' | 'error' | 'info') => void;
  hideToast: () => void;
}

const ToastContext = createContext<ToastContextType | undefined>(undefined);

export function ToastProvider({ children }: { children: ReactNode }) {
  const [message, setMessage] = useState('');
  const [type, setType] = useState<'success' | 'error' | 'info'>('success');
  const [show, setShow] = useState(false);

  const showToast = useCallback((msg: string, t: 'success' | 'error' | 'info' = 'success') => {
    setMessage(msg);
    setType(t);
    setShow(true);
    setTimeout(() => setShow(false), 3000);
  }, []);

  const hideToast = useCallback(() => setShow(false), []);

  return (
    <ToastContext.Provider value={{ message, type, show, showToast, hideToast }}>
      {children}
    </ToastContext.Provider>
  );
}

export function useToast() {
  const context = useContext(ToastContext);
  if (!context) throw new Error('useToast must be used within a ToastProvider');
  return context;
}
