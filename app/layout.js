import './globals.css';

export const viewport = {
  themeColor: '#090d16',
  width: 'device-width',
  initialScale: 1,
};

export const metadata = {
  title: 'MiniEdge — Developer Command Center',
  description: 'MiniEdge — Zero-Dependency Local Developer Gateway & Microservices Command Center',
};

export default function RootLayout({ children }) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
