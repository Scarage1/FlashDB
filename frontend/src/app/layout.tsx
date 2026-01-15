import type { Metadata } from 'next';
import './globals.css';

export const metadata: Metadata = {
  title: 'FlashDB — Lightning-Fast Key-Value Store',
  description: 'A Redis-compatible key-value store with blazing fast performance',
  icons: {
    icon: '/favicon.svg',
  },
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en">
      <body className="font-sans">{children}</body>
    </html>
  );
}
