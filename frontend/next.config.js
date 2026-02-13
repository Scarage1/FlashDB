/** @type {import('next').NextConfig} */
const nextConfig = {
  output: 'export',

  poweredByHeader: false,
  reactStrictMode: true,

  turbopack: {
    root: __dirname,
  },

  images: {
    unoptimized: true,
  },
};

module.exports = nextConfig;
