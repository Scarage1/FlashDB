/** @type {import('next').NextConfig} */
const nextConfig = {
  async rewrites() {
    const apiUrl = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';

    return [
      {
        source: '/api/:path*',
        destination: `${apiUrl}/api/:path*`,
      },
    ];
  },

  poweredByHeader: false,
  reactStrictMode: true,

  turbopack: {
    root: __dirname,
  },

  images: {
    domains: [],
  },
};

module.exports = nextConfig;
