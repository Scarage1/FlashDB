/** @type {import('next').NextConfig} */
const nextConfig = {
  // Only use rewrites in development (Vercel handles production via vercel.json)
  async rewrites() {
    // In production, NEXT_PUBLIC_API_URL is set via Vercel environment variables
    const apiUrl = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';
    
    return [
      {
        source: '/api/:path*',
        destination: `${apiUrl}/api/:path*`,
      },
    ];
  },
  
  // Production optimizations
  poweredByHeader: false,
  reactStrictMode: true,
  
  // Image optimization
  images: {
    domains: [],
  },
};

module.exports = nextConfig;
