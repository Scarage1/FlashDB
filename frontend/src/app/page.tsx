'use client';

import Navbar from '@/components/Navbar';
import Hero from '@/components/Hero';
import StatsGrid from '@/components/StatsGrid';
import Console from '@/components/Console';
import KeysBrowser from '@/components/KeysBrowser';
import FeatureLab from '@/components/FeatureLab';
import Documentation from '@/components/Documentation';
import Toast from '@/components/Toast';
import { ToastProvider } from '@/context/ToastContext';

export default function Home() {
  return (
    <ToastProvider>
      <div className="min-h-screen">
        <Navbar />
        <Hero />
        <StatsGrid />
        
        <main className="max-w-6xl mx-auto px-6 pb-20">
          <Console />
          <KeysBrowser />
          <FeatureLab />
          <Documentation />
        </main>
        
        <Toast />
      </div>
    </ToastProvider>
  );
}
