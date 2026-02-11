# FlashDB Frontend

A modern, Apple-inspired UI for FlashDB built with Next.js, React, TypeScript, and Tailwind CSS.

## Features

- 🎨 **Apple-inspired Design** - Clean, modern UI with smooth animations
- ⚡ **Interactive Console** - Execute FlashDB commands with history support
- 🗂️ **Keys Browser** - Visual management of your database keys
- 🧪 **Feature Lab** - Guided operations for strings, TTL, counters, and sorted sets
- 📚 **Documentation** - Built-in help and command reference

## Getting Started

### Prerequisites

- Node.js 18+
- FlashDB server running on port 6379 (HTTP API on 8080)

### Installation

```bash
# Navigate to frontend directory
cd frontend

# Install dependencies
npm install

# Start development server
npm run dev
```

Open [http://localhost:3000](http://localhost:3000) in your browser.

### Build Check

```bash
# Build the frontend
npm run build

# Start local server from build output
npm start
```

## Project Structure

```
frontend/
├── public/              # Static assets
│   ├── logo.svg
│   └── favicon.svg
├── src/
│   ├── app/            # Next.js app router
│   │   ├── globals.css
│   │   ├── layout.tsx
│   │   └── page.tsx
│   ├── components/     # React components
│   │   ├── Console.tsx
│   │   ├── Documentation.tsx
│   │   ├── FeatureLab.tsx
│   │   ├── Hero.tsx
│   │   ├── KeysBrowser.tsx
│   │   ├── Modal.tsx
│   │   ├── Navbar.tsx
│   │   ├── StatsGrid.tsx
│   │   └── Toast.tsx
│   ├── context/        # React context
│   │   └── ToastContext.tsx
│   └── lib/            # Utilities
│       └── api.ts
├── next.config.js
├── tailwind.config.js
├── tsconfig.json
└── package.json
```

## API Proxy

The frontend proxies API requests to the FlashDB backend:

- `/api/*` → `http://localhost:8080/api/*`

Configure this in `next.config.js` if needed.

## Tech Stack

- **Framework**: Next.js 16
- **Language**: TypeScript
- **Styling**: Tailwind CSS
- **Icons**: Lucide React
