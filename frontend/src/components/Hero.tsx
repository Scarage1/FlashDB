'use client';

import { Zap, ArrowRight } from 'lucide-react';

export default function Hero() {
  const scrollTo = (id: string) => {
    const element = document.getElementById(id);
    if (element) {
      element.scrollIntoView({ behavior: 'smooth' });
    }
  };

  return (
    <section className="py-20 px-6 text-center">
      <div className="max-w-4xl mx-auto">
        {/* Badge */}
        <div className="inline-flex items-center gap-2 px-4 py-2 bg-white border border-gray-200 rounded-full shadow-sm mb-8 animate-fade-in">
          <div className="w-5 h-5 bg-gradient-to-br from-blue-500 to-cyan-400 rounded-full flex items-center justify-center">
            <Zap className="w-3 h-3 text-white" />
          </div>
          <span className="text-sm font-medium text-gray-600">Lightning-Fast Performance</span>
        </div>

        {/* Title */}
        <h1 className="text-6xl md:text-7xl font-bold tracking-tight mb-6 animate-slide-up bg-gradient-to-b from-gray-900 to-gray-600 bg-clip-text text-transparent">
          FlashDB Console
        </h1>

        {/* Subtitle */}
        <p className="text-xl text-gray-500 mb-10 animate-slide-up" style={{ animationDelay: '0.1s' }}>
          Redis-compatible key-value store with blazing fast performance
        </p>

        {/* Actions */}
        <div className="flex flex-wrap gap-4 justify-center animate-slide-up" style={{ animationDelay: '0.2s' }}>
          <button
            onClick={() => scrollTo('console')}
            className="group flex items-center gap-2 px-8 py-4 bg-gradient-to-r from-blue-600 to-cyan-500 text-white font-semibold rounded-full shadow-lg shadow-blue-500/25 hover:shadow-xl hover:shadow-blue-500/30 hover:-translate-y-0.5 transition-all"
          >
            <span>Open Console</span>
            <ArrowRight className="w-5 h-5 group-hover:translate-x-1 transition-transform" />
          </button>
          <button
            onClick={() => scrollTo('docs')}
            className="flex items-center gap-2 px-8 py-4 bg-white text-gray-700 font-semibold rounded-full border border-gray-200 shadow-sm hover:bg-gray-50 hover:-translate-y-0.5 transition-all"
          >
            Documentation
          </button>
        </div>
      </div>
    </section>
  );
}
