import './globals.css';
import { Providers } from '@/app/providers';

export const metadata = {
  title: 'scrumX',
  description: 'Production-grade Scrum SaaS frontend'
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <body>
        <Providers>{children}</Providers>
      </body>
    </html>
  );
}
