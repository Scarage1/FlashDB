'use client';

import { useToast } from '@/context/ToastContext';
import { CheckCircle, XCircle, Info } from 'lucide-react';

export default function Toast() {
  const { message, type, show } = useToast();

  const icons = {
    success: <CheckCircle className="w-5 h-5 text-green-500" />,
    error: <XCircle className="w-5 h-5 text-red-500" />,
    info: <Info className="w-5 h-5 text-blue-500" />,
  };

  const borderColors = {
    success: 'border-green-500',
    error: 'border-red-500',
    info: 'border-blue-500',
  };

  return (
    <div
      className={`fixed bottom-8 right-8 flex items-center gap-3 px-5 py-4 bg-white border ${borderColors[type]} rounded-2xl shadow-2xl z-50 transition-all duration-300 ${
        show ? 'translate-y-0 opacity-100' : 'translate-y-20 opacity-0 pointer-events-none'
      }`}
    >
      {icons[type]}
      <span className="font-medium">{message}</span>
    </div>
  );
}
